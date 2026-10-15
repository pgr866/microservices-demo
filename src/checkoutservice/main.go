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
	"net"
	"os"
	"strings"
	"time"
	"uuid"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/status"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/checkoutservice/genproto"
	money "github.com/GoogleCloudPlatform/microservices-demo/src/checkoutservice/money"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const listenPort = "5050"

var log *logrus.Logger

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
	log.AddHook(loggerName("checkoutservice"))
	level, err := parseLogLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		log.Fatal(err)
	}
	log.Level = level
}

// loggerName adds the "logger" field to every log line.
type loggerName string

func (loggerName) Levels() []logrus.Level { return logrus.AllLevels }

func (n loggerName) Fire(entry *logrus.Entry) error {
	entry.Data["logger"] = string(n)
	return nil
}

// parseLogLevel accepts only the levels shared by every service (logrus.ParseLevel also takes trace, fatal, panic).
func parseLogLevel(s string) (logrus.Level, error) {
	switch strings.ToLower(cmp.Or(s, "info")) {
	case "debug":
		return logrus.DebugLevel, nil
	case "info":
		return logrus.InfoLevel, nil
	case "warn":
		return logrus.WarnLevel, nil
	case "error":
		return logrus.ErrorLevel, nil
	}
	return 0, fmt.Errorf("invalid LOG_LEVEL %q: want debug, info, warn or error", s)
}

type checkoutService struct {
	pb.UnimplementedCheckoutServiceServer

	productCatalogSvc pb.ProductCatalogServiceClient
	cartSvc           pb.CartServiceClient
	currencySvc       pb.CurrencyServiceClient
	shippingSvc       pb.ShippingServiceClient
	emailSvc          pb.EmailServiceClient
	paymentSvc        pb.PaymentServiceClient
}

func main() {
	port := listenPort
	if (os.Getenv("PORT") != "") == true {
		port = os.Getenv("PORT")
	}

	svc := &checkoutService{
		shippingSvc:       pb.NewShippingServiceClient(mustConnGRPC("SHIPPING_SERVICE_ADDR")),
		productCatalogSvc: pb.NewProductCatalogServiceClient(mustConnGRPC("PRODUCT_CATALOG_SERVICE_ADDR")),
		cartSvc:           pb.NewCartServiceClient(mustConnGRPC("CART_SERVICE_ADDR")),
		currencySvc:       pb.NewCurrencyServiceClient(mustConnGRPC("CURRENCY_SERVICE_ADDR")),
		emailSvc:          pb.NewEmailServiceClient(mustConnGRPC("EMAIL_SERVICE_ADDR")),
		paymentSvc:        pb.NewPaymentServiceClient(mustConnGRPC("PAYMENT_SERVICE_ADDR")),
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatal(err)
	}

	srv := grpc.NewServer()

	pb.RegisterCheckoutServiceServer(srv, svc)
	healthcheck := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthcheck)
	log.Infof("starting to listen on tcp: %q", lis.Addr().String())
	if err := serveUntilSignal(srv, lis, healthcheck); err != nil {
		log.Fatal(err)
	}
}

// mustConnGRPC connects lazily: nothing is dialed until the first RPC.
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

func (cs *checkoutService) PlaceOrder(ctx context.Context, req *pb.PlaceOrderRequest) (*pb.PlaceOrderResponse, error) {
	// No user ID: it is the session ID, the key to the cart.
	log.Debugf("[PlaceOrder] user_currency=%q", req.GetUserCurrency())

	if err := validatePlaceOrderRequest(req); err != nil {
		return nil, err
	}

	prep, err := cs.prepareOrderItemsAndShippingQuoteFromCart(ctx, req.GetUserId(), req.GetUserCurrency(), req.GetAddress())
	if err != nil {
		return nil, err
	}

	total, err := orderTotal(req.GetUserCurrency(), prep)
	if err != nil {
		log.Errorf("failed to calculate order total: %v", err)
		return nil, status.Error(codes.Internal, "failed to calculate order total")
	}

	txID, err := cs.chargeCard(ctx, total, req.GetCreditCard())
	if err != nil {
		return nil, err
	}
	// Info level: if shipping fails next, the card stays charged and this is the only record.
	log.Infof("payment went through (transaction_id: %s)", txID)

	shippingTrackingID, err := cs.shipOrder(ctx, req.GetAddress(), prep.cartItems)
	if err != nil {
		return nil, err
	}

	if err := cs.emptyUserCart(ctx, req.GetUserId()); err != nil {
		log.Warnf("%v", err)
	}

	orderResult := &pb.OrderResult{
		OrderId:            uuid.New().String(),
		ShippingTrackingId: shippingTrackingID,
		ShippingCost:       prep.shippingCostLocalized,
		ShippingAddress:    req.GetAddress(),
		Items:              prep.orderItems,
	}

	// By order ID: the email address is personal data.
	if err := cs.sendOrderConfirmation(ctx, req.GetEmail(), orderResult); err != nil {
		log.Warnf("failed to send the confirmation of order %s: %v", orderResult.GetOrderId(), err)
	} else {
		log.Debugf("confirmation of order %s sent", orderResult.GetOrderId())
	}
	resp := &pb.PlaceOrderResponse{Order: orderResult}
	return resp, nil
}

// validatePlaceOrderRequest rejects missing required fields; email is optional (confirmation is best effort).
func validatePlaceOrderRequest(req *pb.PlaceOrderRequest) error {
	switch {
	case req.GetUserId() == "":
		return status.Error(codes.InvalidArgument, "user_id is required")
	case req.GetUserCurrency() == "":
		return status.Error(codes.InvalidArgument, "user_currency is required")
	case req.GetAddress() == nil:
		return status.Error(codes.InvalidArgument, "address is required")
	case req.GetCreditCard() == nil:
		return status.Error(codes.InvalidArgument, "credit_card is required")
	}
	return nil
}

// dependencyError maps a dependency's error to Unavailable/Internal (or the caller's own
// Canceled/DeadlineExceeded); its code and message are logged, never forwarded.
func dependencyError(ctx context.Context, err error, msg string) error {
	code := codes.Internal
	switch {
	case errors.Is(ctx.Err(), context.Canceled):
		code = codes.Canceled
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		code = codes.DeadlineExceeded
	default:
		switch status.Code(err) {
		case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted:
			code = codes.Unavailable
		}
	}
	log.Warnf("%s: %v", msg, err)
	return status.Error(code, msg)
}

// orderTotal fails on an invalid or mixed-currency amount instead of charging a wrong total.
func orderTotal(currency string, prep orderPrep) (*pb.Money, error) {
	total, err := money.Sum(&pb.Money{CurrencyCode: currency}, prep.shippingCostLocalized)
	if err != nil {
		return nil, err
	}
	for _, it := range prep.orderItems {
		itemTotal, err := money.MultiplySlow(it.GetCost(), uint32(it.GetItem().GetQuantity()))
		if err != nil {
			return nil, err
		}
		if total, err = money.Sum(total, itemTotal); err != nil {
			return nil, err
		}
	}
	return total, nil
}

type orderPrep struct {
	orderItems            []*pb.OrderItem
	cartItems             []*pb.CartItem
	shippingCostLocalized *pb.Money
}

func (cs *checkoutService) prepareOrderItemsAndShippingQuoteFromCart(ctx context.Context, userID, userCurrency string, address *pb.Address) (orderPrep, error) {
	var out orderPrep
	cartItems, err := cs.getUserCart(ctx, userID)
	if err != nil {
		return out, err
	}
	// Otherwise the order would charge only shipping for an empty order.
	if len(cartItems) == 0 {
		return out, status.Error(codes.FailedPrecondition, "cart is empty")
	}
	orderItems, err := cs.prepOrderItems(ctx, cartItems, userCurrency)
	if err != nil {
		return out, err
	}
	shippingUSD, err := cs.quoteShipping(ctx, address, cartItems)
	if err != nil {
		return out, err
	}
	shippingPrice, err := cs.convertCurrency(ctx, shippingUSD, userCurrency)
	if err != nil {
		return out, err
	}

	out.shippingCostLocalized = shippingPrice
	out.cartItems = cartItems
	out.orderItems = orderItems
	return out, nil
}

func (cs *checkoutService) quoteShipping(ctx context.Context, address *pb.Address, items []*pb.CartItem) (*pb.Money, error) {
	shippingQuote, err := cs.shippingSvc.GetQuote(ctx, &pb.GetQuoteRequest{
		Address: address,
		Items:   items})
	if err != nil {
		return nil, dependencyError(ctx, err, "failed to get shipping quote")
	}
	return shippingQuote.GetCostUsd(), nil
}

func (cs *checkoutService) getUserCart(ctx context.Context, userID string) ([]*pb.CartItem, error) {
	cart, err := cs.cartSvc.GetCart(ctx, &pb.GetCartRequest{UserId: userID})
	if err != nil {
		return nil, dependencyError(ctx, err, "failed to get user cart")
	}
	return cart.GetItems(), nil
}

func (cs *checkoutService) emptyUserCart(ctx context.Context, userID string) error {
	if _, err := cs.cartSvc.EmptyCart(ctx, &pb.EmptyCartRequest{UserId: userID}); err != nil {
		return fmt.Errorf("failed to empty user cart during checkout: %w", err)
	}
	return nil
}

// maxConcurrentRPCs covers the whole catalog but stops a huge cart opening hundreds of streams.
const maxConcurrentRPCs = 10

// prepOrderItems prices items concurrently, keeping cart order; the first error cancels the rest.
func (cs *checkoutService) prepOrderItems(ctx context.Context, items []*pb.CartItem, userCurrency string) ([]*pb.OrderItem, error) {
	// MultiplySlow takes uint32: a negative quantity would loop billions of times, 0 would charge one unit.
	for _, item := range items {
		if item.GetQuantity() < 1 {
			return nil, status.Errorf(codes.FailedPrecondition, "cart has an invalid quantity %d for product %q", item.GetQuantity(), item.GetProductId())
		}
	}

	out := make([]*pb.OrderItem, len(items))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrentRPCs)
	for i, item := range items {
		g.Go(func() error {
			product, err := cs.productCatalogSvc.GetProduct(gctx, &pb.GetProductRequest{Id: item.GetProductId()})
			if status.Code(err) == codes.NotFound {
				return status.Errorf(codes.FailedPrecondition, "cart has product %q, which is not in the catalog", item.GetProductId())
			}
			if err != nil {
				return dependencyError(gctx, err, fmt.Sprintf("failed to get product %q", item.GetProductId()))
			}
			price, err := cs.convertCurrency(gctx, product.GetPriceUsd(), userCurrency)
			if err != nil {
				return err
			}
			out[i] = &pb.OrderItem{
				Item: item,
				Cost: price}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return out, nil
}

func (cs *checkoutService) convertCurrency(ctx context.Context, from *pb.Money, toCurrency string) (*pb.Money, error) {
	result, err := cs.currencySvc.Convert(ctx, &pb.CurrencyConversionRequest{
		From:   from,
		ToCode: toCurrency})
	// Prices are always USD, so InvalidArgument can only be the user's currency.
	if status.Code(err) == codes.InvalidArgument {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported user_currency %q", toCurrency)
	}
	if err != nil {
		return nil, dependencyError(ctx, err, fmt.Sprintf("failed to convert currency to %s", toCurrency))
	}
	return result, nil
}

func (cs *checkoutService) chargeCard(ctx context.Context, amount *pb.Money, paymentInfo *pb.CreditCardInfo) (string, error) {
	paymentResp, err := cs.paymentSvc.Charge(ctx, &pb.ChargeRequest{
		Amount:     amount,
		CreditCard: paymentInfo})
	// An invalid or expired card is the caller's error.
	if status.Code(err) == codes.InvalidArgument {
		return "", status.Error(codes.InvalidArgument, status.Convert(err).Message())
	}
	if err != nil {
		return "", dependencyError(ctx, err, "failed to charge card")
	}
	return paymentResp.GetTransactionId(), nil
}

func (cs *checkoutService) sendOrderConfirmation(ctx context.Context, email string, order *pb.OrderResult) error {
	_, err := cs.emailSvc.SendOrderConfirmation(ctx, &pb.SendOrderConfirmationRequest{
		Email: email,
		Order: order})
	return err
}

func (cs *checkoutService) shipOrder(ctx context.Context, address *pb.Address, items []*pb.CartItem) (string, error) {
	resp, err := cs.shippingSvc.ShipOrder(ctx, &pb.ShipOrderRequest{
		Address: address,
		Items:   items})
	if err != nil {
		return "", dependencyError(ctx, err, "failed to ship order")
	}
	return resp.GetTrackingId(), nil
}
