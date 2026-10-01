// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/frontend/genproto"
)

const (
	port                  = "8082"
	defaultCurrency       = "USD"
	defaultRequestTimeout = 5 * time.Second
	// shutdownTimeout is how long in-flight requests get to finish on SIGTERM:
	// less than the 30 s Kubernetes waits by default before sending SIGKILL.
	shutdownTimeout = 10 * time.Second
	cookieMaxAge    = 60 * 60 * 48

	cookiePrefix    = "shop_"
	cookieSessionID = cookiePrefix + "session-id"
	cookieCurrency  = cookiePrefix + "currency"
)

var (
	whitelistedCurrencies = map[string]bool{
		"USD": true,
		"EUR": true,
		"CAD": true,
		"JPY": true,
		"GBP": true,
		"TRY": true,
	}

	baseUrl = ""

	log *logrus.Logger
)

func init() {
	log = logrus.New()
	log.Formatter = &logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "severity",
			logrus.FieldKeyMsg:   "message",
		},
		TimestampFormat: time.RFC3339Nano,
	}
	log.Out = os.Stdout
	level, err := logrus.ParseLevel(cmp.Or(os.Getenv("LOG_LEVEL"), "info"))
	if err != nil {
		log.Fatalf("invalid LOG_LEVEL: %v", err)
	}
	log.Level = level
}

type ctxKeySessionID struct{}

type frontendServer struct {
	productCatalogSvc pb.ProductCatalogServiceClient
	currencySvc       pb.CurrencyServiceClient
	cartSvc           pb.CartServiceClient
	recommendationSvc pb.RecommendationServiceClient
	checkoutSvc       pb.CheckoutServiceClient
	shippingSvc       pb.ShippingServiceClient
	adSvc             pb.AdServiceClient
}

func main() {
	baseUrl = os.Getenv("BASE_URL")

	srvPort := port
	if os.Getenv("PORT") != "" {
		srvPort = os.Getenv("PORT")
	}
	addr := os.Getenv("LISTEN_ADDR")
	requestTimeout := defaultRequestTimeout
	if v := os.Getenv("REQUEST_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			log.Fatalf("invalid REQUEST_TIMEOUT %q: want a positive duration such as \"5s\"", v)
		}
		requestTimeout = d
	}
	loadPlatformDetails(os.Getenv("ENV_PLATFORM"))
	loadDeploymentDetails()

	svc := &frontendServer{
		productCatalogSvc: pb.NewProductCatalogServiceClient(mustConnGRPC("PRODUCT_CATALOG_SERVICE_ADDR")),
		currencySvc:       pb.NewCurrencyServiceClient(mustConnGRPC("CURRENCY_SERVICE_ADDR")),
		cartSvc:           pb.NewCartServiceClient(mustConnGRPC("CART_SERVICE_ADDR")),
		recommendationSvc: pb.NewRecommendationServiceClient(mustConnGRPC("RECOMMENDATION_SERVICE_ADDR")),
		checkoutSvc:       pb.NewCheckoutServiceClient(mustConnGRPC("CHECKOUT_SERVICE_ADDR")),
		shippingSvc:       pb.NewShippingServiceClient(mustConnGRPC("SHIPPING_SERVICE_ADDR")),
		adSvc:             pb.NewAdServiceClient(mustConnGRPC("AD_SERVICE_ADDR")),
	}

	srv := newServer(addr+":"+srvPort, newHandler(svc, log, requestTimeout), requestTimeout)
	log.Infof("starting server on %s:%s (request timeout %s)", addr, srvPort, requestTimeout)
	if err := serveUntilSignal(srv); err != nil {
		log.Fatal(err)
	}
}

// serveUntilSignal serves until SIGTERM (what Kubernetes sends to delete a pod)
// or SIGINT, then stops accepting connections, lets in-flight requests finish
// (up to shutdownTimeout, then closes the rest) and returns nil.
func serveUntilSignal(srv *http.Server) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	stopped := make(chan struct{})
	go func() {
		<-ctx.Done()
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Warnf("requests still in flight after %v, cutting them: %v", shutdownTimeout, err)
			_ = srv.Close()
		}
		close(stopped)
	}()

	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	// ListenAndServe returns as soon as Shutdown starts: wait for the requests
	// in flight.
	<-stopped
	return nil
}

func newHandler(svc *frontendServer, logger *logrus.Logger, requestTimeout time.Duration) http.Handler {
	handler := newRouter(svc)
	handler = withTimeout(handler, requestTimeout)
	// Rejects form posts sent by another site's page (CSRF), on top of the
	// SameSite cookies. Requests without browser headers (curl, k6) still pass.
	handler = http.NewCrossOriginProtection().Handler(handler)
	handler = securityHeaders(handler)
	handler = limitBody(handler)
	handler = &logHandler{log: logger, next: handler}
	handler = ensureSessionID(handler)
	return handler
}

// newServer returns the HTTP server, with timeouts so that a slow or idle
// client can't hold a connection open forever.
func newServer(addr string, handler http.Handler, requestTimeout time.Duration) *http.Server {
	return &http.Server{
		Addr:    addr,
		Handler: handler,
		// Reading the headers slowly is the Slowloris attack.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		// Longer than the request deadline, so the 504 page can still be sent.
		WriteTimeout: requestTimeout + 5*time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

// newRouter maps every route of the shop to its handler. A "GET" pattern also
// matches HEAD requests, and a request with another method gets a 405.
func newRouter(svc *frontendServer) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+baseUrl+"/{$}", svc.homeHandler)
	mux.HandleFunc("GET "+baseUrl+"/product/{id}", svc.productHandler)
	mux.HandleFunc("GET "+baseUrl+"/cart", svc.viewCartHandler)
	mux.HandleFunc("POST "+baseUrl+"/cart", svc.addToCartHandler)
	mux.HandleFunc("POST "+baseUrl+"/cart/empty", svc.emptyCartHandler)
	mux.HandleFunc("POST "+baseUrl+"/setCurrency", svc.setCurrencyHandler)
	mux.HandleFunc("GET "+baseUrl+"/logout", svc.logoutHandler)
	mux.HandleFunc("POST "+baseUrl+"/cart/checkout", svc.placeOrderHandler)
	mux.Handle("GET "+baseUrl+"/static/", http.StripPrefix(baseUrl+"/static/", noDirListing(http.FileServer(http.Dir("./static/")))))
	// Nothing to do if writing fails: the client is already gone.
	mux.HandleFunc(baseUrl+"/robots.txt", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "User-agent: *\nDisallow: /") })
	mux.HandleFunc(baseUrl+"/_healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, fmt.Sprintf("ok")) })
	return mux
}

// noDirListing answers 404 for a directory instead of letting http.FileServer
// list the files in it.
func noDirListing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "" || strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// mustConnGRPC connects lazily to the address in envKey: nothing is dialed
// until the first RPC.
func mustConnGRPC(envKey string) *grpc.ClientConn {
	addr := os.Getenv(envKey)
	if addr == "" {
		log.Fatalf("environment variable %q not set", envKey)
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("grpc: failed to connect %s: %v", addr, err)
	}
	log.Infof("%s: %s", envKey, addr)
	return conn
}
