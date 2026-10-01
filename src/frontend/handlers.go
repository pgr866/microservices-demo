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
	"context"
	"errors"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/frontend/genproto"
	"github.com/GoogleCloudPlatform/microservices-demo/src/frontend/money"
	"github.com/GoogleCloudPlatform/microservices-demo/src/frontend/validator"
)

type platformDetails struct {
	css      string
	provider string
}

var (
	frontendMessage = strings.TrimSpace(os.Getenv("FRONTEND_MESSAGE"))
	isCymbalBrand   = strings.ToLower(os.Getenv("CYMBAL_BRANDING")) == "true"
	templates       = template.Must(template.New("").
			Funcs(template.FuncMap{
			"renderMoney":        renderMoney,
			"renderCurrencyLogo": renderCurrencyLogo,
		}).ParseGlob("templates/*.html"))
	plat platformDetails
)

var validEnvs = []string{"local", "gcp", "azure", "aws", "onprem", "alibaba"}

func (fe *frontendServer) homeHandler(w http.ResponseWriter, r *http.Request) {
	log := r.Context().Value(ctxKeyLog{}).(logrus.FieldLogger)
	log.WithField("currency", currentCurrency(r)).Info("home")
	currencies, err := fe.getCurrencies(r.Context())
	if err != nil {
		renderHTTPError(log, r, w, fmt.Errorf("could not retrieve currencies: %w", err), http.StatusInternalServerError)
		return
	}
	products, err := fe.getProducts(r.Context())
	if err != nil {
		renderHTTPError(log, r, w, fmt.Errorf("could not retrieve products: %w", err), http.StatusInternalServerError)
		return
	}
	cart, err := fe.getCart(r.Context(), sessionID(r))
	if err != nil {
		renderHTTPError(log, r, w, fmt.Errorf("could not retrieve cart: %w", err), http.StatusInternalServerError)
		return
	}

	type productView struct {
		Item  *pb.Product
		Price *pb.Money
	}
	ps := make([]productView, len(products))
	for i, p := range products {
		price, err := fe.convertCurrency(r.Context(), p.GetPriceUsd(), currentCurrency(r))
		if err != nil {
			renderHTTPError(log, r, w, fmt.Errorf("failed to do currency conversion for product %s: %w", p.GetId(), err), http.StatusInternalServerError)
			return
		}
		ps[i] = productView{p, price}
	}

	if err := templates.ExecuteTemplate(w, "home", injectCommonTemplateData(r, map[string]interface{}{
		"show_currency": true,
		"currencies":    currencies,
		"products":      ps,
		"cart_size":     cartSize(cart),
		"banner_color":  os.Getenv("BANNER_COLOR"), // illustrates canary deployments
		"ad":            fe.chooseAd(r.Context(), []string{}, log),
	})); err != nil {
		log.Error(err)
	}
}

// loadPlatformDetails sets the platform badge from ENV_PLATFORM, once at
// startup: setting it on every request, as before, made concurrent requests
// write the same global variable at the same time.
func loadPlatformDetails(env string) {
	env = strings.ToLower(env)
	if !stringinSlice(validEnvs, env) {
		log.Infof("ENV_PLATFORM %q is empty or invalid, using \"local\"", env)
		env = "local"
	}
	plat.setPlatformDetails(env)
}

func (plat *platformDetails) setPlatformDetails(env string) {
	switch env {
	case "aws":
		plat.provider = "AWS"
		plat.css = "aws-platform"
	case "onprem":
		plat.provider = "On-Premises"
		plat.css = "onprem-platform"
	case "azure":
		plat.provider = "Azure"
		plat.css = "azure-platform"
	case "gcp":
		plat.provider = "Google Cloud"
		plat.css = "gcp-platform"
	case "alibaba":
		plat.provider = "Alibaba Cloud"
		plat.css = "alibaba-platform"
	default:
		plat.provider = "local"
		plat.css = "local"
	}
}

func (fe *frontendServer) productHandler(w http.ResponseWriter, r *http.Request) {
	log := r.Context().Value(ctxKeyLog{}).(logrus.FieldLogger)
	id := r.PathValue("id")
	log.WithField("id", id).WithField("currency", currentCurrency(r)).
		Debug("serving product page")

	p, err := fe.getProduct(r.Context(), id)
	if err != nil {
		renderHTTPError(log, r, w, fmt.Errorf("could not retrieve product: %w", err), http.StatusInternalServerError)
		return
	}
	currencies, err := fe.getCurrencies(r.Context())
	if err != nil {
		renderHTTPError(log, r, w, fmt.Errorf("could not retrieve currencies: %w", err), http.StatusInternalServerError)
		return
	}

	cart, err := fe.getCart(r.Context(), sessionID(r))
	if err != nil {
		renderHTTPError(log, r, w, fmt.Errorf("could not retrieve cart: %w", err), http.StatusInternalServerError)
		return
	}

	price, err := fe.convertCurrency(r.Context(), p.GetPriceUsd(), currentCurrency(r))
	if err != nil {
		renderHTTPError(log, r, w, fmt.Errorf("failed to convert currency: %w", err), http.StatusInternalServerError)
		return
	}

	// ignores the error retrieving recommendations since it is not critical
	recommendations, err := fe.getRecommendations(r.Context(), sessionID(r), []string{id})
	if err != nil {
		log.WithField("error", err).Warn("failed to get product recommendations")
	}

	product := struct {
		Item  *pb.Product
		Price *pb.Money
	}{p, price}

	if err := templates.ExecuteTemplate(w, "product", injectCommonTemplateData(r, map[string]interface{}{
		"ad":              fe.chooseAd(r.Context(), p.Categories, log),
		"show_currency":   true,
		"currencies":      currencies,
		"product":         product,
		"recommendations": recommendations,
		"cart_size":       cartSize(cart),
	})); err != nil {
		log.Error(err)
	}
}

func (fe *frontendServer) addToCartHandler(w http.ResponseWriter, r *http.Request) {
	log := r.Context().Value(ctxKeyLog{}).(logrus.FieldLogger)
	quantity, _ := strconv.ParseUint(r.FormValue("quantity"), 10, 32)
	productID := r.FormValue("product_id")
	payload := validator.AddToCartPayload{
		Quantity:  quantity,
		ProductID: productID,
	}
	if err := payload.Validate(); err != nil {
		renderHTTPError(log, r, w, validator.ValidationErrorResponse(err), http.StatusUnprocessableEntity)
		return
	}
	log.WithField("product", payload.ProductID).WithField("quantity", payload.Quantity).Debug("adding to cart")

	p, err := fe.getProduct(r.Context(), payload.ProductID)
	if err != nil {
		renderHTTPError(log, r, w, fmt.Errorf("could not retrieve product: %w", err), http.StatusInternalServerError)
		return
	}

	if err := fe.insertCart(r.Context(), sessionID(r), p.GetId(), int32(payload.Quantity)); err != nil {
		renderHTTPError(log, r, w, fmt.Errorf("failed to add to cart: %w", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("location", baseUrl+"/cart")
	w.WriteHeader(http.StatusFound)
}

func (fe *frontendServer) emptyCartHandler(w http.ResponseWriter, r *http.Request) {
	log := r.Context().Value(ctxKeyLog{}).(logrus.FieldLogger)
	log.Debug("emptying cart")

	if err := fe.emptyCart(r.Context(), sessionID(r)); err != nil {
		renderHTTPError(log, r, w, fmt.Errorf("failed to empty cart: %w", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("location", baseUrl+"/")
	w.WriteHeader(http.StatusFound)
}

func (fe *frontendServer) viewCartHandler(w http.ResponseWriter, r *http.Request) {
	log := r.Context().Value(ctxKeyLog{}).(logrus.FieldLogger)
	log.Debug("view user cart")
	currencies, err := fe.getCurrencies(r.Context())
	if err != nil {
		renderHTTPError(log, r, w, fmt.Errorf("could not retrieve currencies: %w", err), http.StatusInternalServerError)
		return
	}
	cart, err := fe.getCart(r.Context(), sessionID(r))
	if err != nil {
		renderHTTPError(log, r, w, fmt.Errorf("could not retrieve cart: %w", err), http.StatusInternalServerError)
		return
	}

	// ignores the error retrieving recommendations since it is not critical
	recommendations, err := fe.getRecommendations(r.Context(), sessionID(r), cartIDs(cart))
	if err != nil {
		log.WithField("error", err).Warn("failed to get product recommendations")
	}

	shippingCost, err := fe.getShippingQuote(r.Context(), cart, currentCurrency(r))
	if err != nil {
		renderHTTPError(log, r, w, fmt.Errorf("failed to get shipping quote: %w", err), http.StatusInternalServerError)
		return
	}

	type cartItemView struct {
		Item     *pb.Product
		Quantity int32
		Price    *pb.Money
	}
	items := make([]cartItemView, len(cart))
	totalPrice := &pb.Money{CurrencyCode: currentCurrency(r)}
	for i, item := range cart {
		p, err := fe.getProduct(r.Context(), item.GetProductId())
		if err != nil {
			renderHTTPError(log, r, w, fmt.Errorf("could not retrieve product #%s: %w", item.GetProductId(), err), http.StatusInternalServerError)
			return
		}
		price, err := fe.convertCurrency(r.Context(), p.GetPriceUsd(), currentCurrency(r))
		if err != nil {
			renderHTTPError(log, r, w, fmt.Errorf("could not convert currency for product #%s: %w", item.GetProductId(), err), http.StatusInternalServerError)
			return
		}

		multPrice, err := money.MultiplySlow(price, uint32(item.GetQuantity()))
		if err == nil {
			totalPrice, err = money.Sum(totalPrice, multPrice)
		}
		if err != nil {
			renderHTTPError(log, r, w, fmt.Errorf("could not calculate the price of product #%s: %w", item.GetProductId(), err), http.StatusInternalServerError)
			return
		}
		items[i] = cartItemView{
			Item:     p,
			Quantity: item.GetQuantity(),
			Price:    multPrice}
	}
	totalPrice, err = money.Sum(totalPrice, shippingCost)
	if err != nil {
		renderHTTPError(log, r, w, fmt.Errorf("could not add the shipping cost: %w", err), http.StatusInternalServerError)
		return
	}
	year := time.Now().Year()

	if err := templates.ExecuteTemplate(w, "cart", injectCommonTemplateData(r, map[string]interface{}{
		"currencies":       currencies,
		"recommendations":  recommendations,
		"cart_size":        cartSize(cart),
		"shipping_cost":    shippingCost,
		"show_currency":    true,
		"total_cost":       totalPrice,
		"items":            items,
		"expiration_years": []int{year, year + 1, year + 2, year + 3, year + 4},
	})); err != nil {
		log.Error(err)
	}
}

func (fe *frontendServer) placeOrderHandler(w http.ResponseWriter, r *http.Request) {
	log := r.Context().Value(ctxKeyLog{}).(logrus.FieldLogger)
	log.Debug("placing order")

	var (
		email         = r.FormValue("email")
		streetAddress = r.FormValue("street_address")
		zipCode, _    = strconv.ParseInt(r.FormValue("zip_code"), 10, 32)
		city          = r.FormValue("city")
		state         = r.FormValue("state")
		country       = r.FormValue("country")
		ccNumber      = r.FormValue("credit_card_number")
		ccMonth, _    = strconv.ParseInt(r.FormValue("credit_card_expiration_month"), 10, 32)
		ccYear, _     = strconv.ParseInt(r.FormValue("credit_card_expiration_year"), 10, 32)
		ccCVV, _      = strconv.ParseInt(r.FormValue("credit_card_cvv"), 10, 32)
	)

	payload := validator.PlaceOrderPayload{
		Email:         email,
		StreetAddress: streetAddress,
		ZipCode:       zipCode,
		City:          city,
		State:         state,
		Country:       country,
		CcNumber:      ccNumber,
		CcMonth:       ccMonth,
		CcYear:        ccYear,
		CcCVV:         ccCVV,
	}
	if err := payload.Validate(); err != nil {
		renderHTTPError(log, r, w, validator.ValidationErrorResponse(err), http.StatusUnprocessableEntity)
		return
	}

	order, err := fe.checkoutSvc.PlaceOrder(r.Context(), &pb.PlaceOrderRequest{
		Email: payload.Email,
		CreditCard: &pb.CreditCardInfo{
			CreditCardNumber:          payload.CcNumber,
			CreditCardExpirationMonth: int32(payload.CcMonth),
			CreditCardExpirationYear:  int32(payload.CcYear),
			CreditCardCvv:             int32(payload.CcCVV)},
		UserId:       sessionID(r),
		UserCurrency: currentCurrency(r),
		Address: &pb.Address{
			StreetAddress: payload.StreetAddress,
			City:          payload.City,
			State:         payload.State,
			ZipCode:       int32(payload.ZipCode),
			Country:       payload.Country},
	})
	if err != nil {
		renderHTTPError(log, r, w, fmt.Errorf("failed to complete the order: %w", err), http.StatusInternalServerError)
		return
	}
	log.WithField("order", order.GetOrder().GetOrderId()).Info("order placed")

	recommendations, _ := fe.getRecommendations(r.Context(), sessionID(r), nil)

	totalPaid, err := orderTotal(order.GetOrder())
	if err != nil {
		renderHTTPError(log, r, w, fmt.Errorf("could not calculate the total paid: %w", err), http.StatusInternalServerError)
		return
	}

	currencies, err := fe.getCurrencies(r.Context())
	if err != nil {
		renderHTTPError(log, r, w, fmt.Errorf("could not retrieve currencies: %w", err), http.StatusInternalServerError)
		return
	}

	if err := templates.ExecuteTemplate(w, "order", injectCommonTemplateData(r, map[string]interface{}{
		"show_currency":   false,
		"currencies":      currencies,
		"order":           order.GetOrder(),
		"total_paid":      totalPaid,
		"recommendations": recommendations,
	})); err != nil {
		log.Error(err)
	}
}

func (fe *frontendServer) logoutHandler(w http.ResponseWriter, r *http.Request) {
	log := r.Context().Value(ctxKeyLog{}).(logrus.FieldLogger)
	log.Debug("logging out")
	for _, c := range r.Cookies() {
		c.Expires = time.Now().Add(-time.Hour * 24 * 365)
		c.MaxAge = -1
		http.SetCookie(w, c)
	}
	w.Header().Set("Location", baseUrl+"/")
	w.WriteHeader(http.StatusFound)
}

func (fe *frontendServer) setCurrencyHandler(w http.ResponseWriter, r *http.Request) {
	log := r.Context().Value(ctxKeyLog{}).(logrus.FieldLogger)
	cur := r.FormValue("currency_code")
	payload := validator.SetCurrencyPayload{Currency: cur, Allowed: whitelistedCurrencies}
	if err := payload.Validate(); err != nil {
		renderHTTPError(log, r, w, validator.ValidationErrorResponse(err), http.StatusUnprocessableEntity)
		return
	}
	log.WithField("curr.new", payload.Currency).WithField("curr.old", currentCurrency(r)).
		Debug("setting currency")

	http.SetCookie(w, &http.Cookie{
		Name:     cookieCurrency,
		Value:    payload.Currency,
		MaxAge:   cookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Location", sameSiteReferer(r))
	w.WriteHeader(http.StatusFound)
}

// sameSiteReferer returns the page to go back to after changing the currency.
// The Referer header comes from the client, so it is only followed if it points
// to a page of this site: redirecting to it as is would be an open redirect.
func sameSiteReferer(r *http.Request) string {
	ref, err := url.Parse(r.Header.Get("referer"))
	if err != nil || ref.Host != r.Host {
		return baseUrl + "/"
	}
	// RequestURI escapes the path (e.g. "\\" as "%5C"), but a path starting
	// with "//" would still be read by the browser as another host.
	path := ref.RequestURI()
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return baseUrl + "/"
	}
	return path
}

// orderTotal adds the shipping cost and every item's cost times its quantity.
// Summing from zero also validates the shipping cost when there are no items.
func orderTotal(order *pb.OrderResult) (*pb.Money, error) {
	shipping := order.GetShippingCost()
	total, err := money.Sum(&pb.Money{CurrencyCode: shipping.GetCurrencyCode()}, shipping)
	if err != nil {
		return nil, err
	}
	for _, item := range order.GetItems() {
		itemTotal, err := money.MultiplySlow(item.GetCost(), uint32(item.GetItem().GetQuantity()))
		if err != nil {
			return nil, err
		}
		if total, err = money.Sum(total, itemTotal); err != nil {
			return nil, err
		}
	}
	return total, nil
}

// chooseAd queries for advertisements available and randomly chooses one, if
// available. It ignores the error retrieving the ad since it is not critical.
func (fe *frontendServer) chooseAd(ctx context.Context, ctxKeys []string, log logrus.FieldLogger) *pb.Ad {
	ads, err := fe.getAd(ctx, ctxKeys)
	if err != nil {
		log.WithField("error", err).Warn("failed to retrieve ads")
		return nil
	}
	// rand.Intn panics with 0, which took down the whole page.
	if len(ads) == 0 {
		return nil
	}
	return ads[rand.Intn(len(ads))]
}

// renderHTTPError renders the error page. If err comes from a failed gRPC call,
// its code decides the HTTP status instead of the handler's default one. The
// page only shows the reason for a client error (e.g. an expired card): for a
// server error the details stay in the log, found by the request ID shown on
// the page, so no internal error chain or address reaches the browser.
func renderHTTPError(log logrus.FieldLogger, r *http.Request, w http.ResponseWriter, err error, code int) {
	st, fromGRPC := grpcStatus(err)
	if fromGRPC {
		code = httpStatusFromCode(st.Code())
	}

	log = log.WithField("error", err.Error())
	message := ""
	if code < http.StatusInternalServerError {
		log.Warn("request error")
		message = err.Error()
		if fromGRPC {
			message = st.Message()
		}
	} else {
		log.Error("request error")
	}

	w.WriteHeader(code)

	if templateErr := templates.ExecuteTemplate(w, "error", injectCommonTemplateData(r, map[string]interface{}{
		"error":       message,
		"status_code": code,
		"status":      http.StatusText(code),
	})); templateErr != nil {
		log.Error(templateErr)
	}
}

// grpcStatus returns the status of the failed gRPC call that err wraps, if any.
// Unlike status.FromError, its message is the service's own, without the
// context added by the wrapping errors.
func grpcStatus(err error) (*status.Status, bool) {
	var se interface{ GRPCStatus() *status.Status }
	if !errors.As(err, &se) {
		return nil, false
	}
	return se.GRPCStatus(), true
}

// httpStatusFromCode maps a gRPC code to its HTTP status, as documented in
// google/rpc/code.proto.
func httpStatusFromCode(c codes.Code) int {
	switch c {
	case codes.OK:
		return http.StatusOK
	case codes.Canceled:
		return 499 // client closed the request; no constant in net/http
	case codes.InvalidArgument, codes.FailedPrecondition, codes.OutOfRange:
		return http.StatusBadRequest
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists, codes.Aborted:
		return http.StatusConflict
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	default: // Unknown, Internal, DataLoss
		return http.StatusInternalServerError
	}
}

func injectCommonTemplateData(r *http.Request, payload map[string]interface{}) map[string]interface{} {
	data := map[string]interface{}{
		"session_id":        sessionID(r),
		"request_id":        r.Context().Value(ctxKeyRequestID{}),
		"user_currency":     currentCurrency(r),
		"platform_css":      plat.css,
		"platform_name":     plat.provider,
		"is_cymbal_brand":   isCymbalBrand,
		"deploymentDetails": deploymentDetailsMap,
		"frontendMessage":   frontendMessage,
		"currentYear":       time.Now().Year(),
		"baseUrl":           baseUrl,
	}

	for k, v := range payload {
		data[k] = v
	}

	return data
}

func currentCurrency(r *http.Request) string {
	c, _ := r.Cookie(cookieCurrency)
	if c != nil {
		return c.Value
	}
	return defaultCurrency
}

func sessionID(r *http.Request) string {
	v := r.Context().Value(ctxKeySessionID{})
	if v != nil {
		return v.(string)
	}
	return ""
}

func cartIDs(c []*pb.CartItem) []string {
	out := make([]string, len(c))
	for i, v := range c {
		out[i] = v.GetProductId()
	}
	return out
}

// get total # of items in cart
func cartSize(c []*pb.CartItem) int {
	cartSize := 0
	for _, item := range c {
		cartSize += int(item.GetQuantity())
	}
	return cartSize
}

func renderMoney(money *pb.Money) string {
	currencyLogo := renderCurrencyLogo(money.GetCurrencyCode())
	return fmt.Sprintf("%s%d.%02d", currencyLogo, money.GetUnits(), money.GetNanos()/10000000)
}

func renderCurrencyLogo(currencyCode string) string {
	logos := map[string]string{
		"USD": "$",
		"CAD": "$",
		"JPY": "¥",
		"EUR": "€",
		"TRY": "₺",
		"GBP": "£",
	}

	logo := "$" //default
	if val, ok := logos[currencyCode]; ok {
		logo = val
	}
	return logo
}

func stringinSlice(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
