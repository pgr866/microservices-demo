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
	"time"
	"uuid"

	"github.com/sirupsen/logrus"
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
	level, err := logrus.ParseLevel(cmp.Or(os.Getenv("LOG_LEVEL"), "info"))
	if err != nil {
		log.Fatalf("invalid LOG_LEVEL: %v", err)
	}
	log.Level = level
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
	if os.Getenv("PORT") != "" {
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

func (cs *checkoutService) PlaceOrder(ctx context.Context, req *pb.PlaceOrderRequest) (*pb.PlaceOrderResponse, error) {
	// No user ID: it is the session ID frontend keys the cart with, kept out of the logs.
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
		return nil, status.Errorf(codes.Internal, "failed to calculate order total: %v", err)
	}

	txID, err := cs.chargeCard(ctx, total, req.GetCreditCard())
	if err != nil {
		return nil, err
	}
	// Info, unlike the rest of the order: if shipping fails next, the order
	// fails but the card stays charged, and this is the record of it.
	log.Infof("payment went through (transaction_id: %s)", txID)

	shippingTrackingID, err := cs.shipOrder(ctx, req.GetAddress(), prep.cartItems)
	if err != nil {
		return nil, err
	}

	if err := cs.emptyUserCart(ctx, req.GetUserId()); err != nil {
		log.Warnf("%v", err)
	}

	orderResult := &pb.OrderResult{
		OrderId:            fmt.Sprintf("%s", uuid.New().String()),
		ShippingTrackingId: shippingTrackingID,
		ShippingCost:       prep.shippingCostLocalized,
		ShippingAddress:    req.GetAddress(),
		Items:              prep.orderItems,
	}

	// Logged by order ID: the email address is personal data, kept out of the logs.
	if err := cs.sendOrderConfirmation(ctx, req.GetEmail(), orderResult); err != nil {
		log.Warnf("failed to send the confirmation of order %s: %v", orderResult.GetOrderId(), err)
	} else {
		log.Debugf("confirmation of order %s sent", orderResult.GetOrderId())
	}
	resp := &pb.PlaceOrderResponse{Order: orderResult}
	return resp, nil
}

// validatePlaceOrderRequest rejects a request missing a field the order can't
// be placed without, before calling any other service. The email is optional:
// the confirmation is best effort.
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

// dependencyError maps the error of a call to another service: Unavailable if
// retrying is worth it, Internal otherwise. The dependency's own code isn't
// passed through, since a NotFound from the catalog would look as if the order
// itself did not exist.
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
	return status.Errorf(code, "%s: %v", msg, err)
}

// orderTotal adds the shipping cost and every item's cost times its quantity.
// Fails if the currency service returned an invalid amount or one in another
// currency, instead of charging a wrong total.
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
	// Otherwise the order would charge only the shipping cost and confirm by
	// email an order with nothing in it.
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

func (cs *checkoutService) prepOrderItems(ctx context.Context, items []*pb.CartItem, userCurrency string) ([]*pb.OrderItem, error) {
	out := make([]*pb.OrderItem, len(items))

	for i, item := range items {
		// MultiplySlow takes the quantity as uint32: a negative one would loop
		// billions of times, and 0 would still charge one unit.
		if item.GetQuantity() < 1 {
			return nil, status.Errorf(codes.FailedPrecondition, "cart has an invalid quantity %d for product %q", item.GetQuantity(), item.GetProductId())
		}
		product, err := cs.productCatalogSvc.GetProduct(ctx, &pb.GetProductRequest{Id: item.GetProductId()})
		if status.Code(err) == codes.NotFound {
			return nil, status.Errorf(codes.FailedPrecondition, "cart has product %q, which is not in the catalog", item.GetProductId())
		}
		if err != nil {
			return nil, dependencyError(ctx, err, fmt.Sprintf("failed to get product %q", item.GetProductId()))
		}
		price, err := cs.convertCurrency(ctx, product.GetPriceUsd(), userCurrency)
		if err != nil {
			return nil, err
		}
		out[i] = &pb.OrderItem{
			Item: item,
			Cost: price}
	}
	return out, nil
}

func (cs *checkoutService) convertCurrency(ctx context.Context, from *pb.Money, toCurrency string) (*pb.Money, error) {
	result, err := cs.currencySvc.Convert(ctx, &pb.CurrencyConversionRequest{
		From:   from,
		ToCode: toCurrency})
	// Prices and shipping costs are always in USD, so an invalid argument can
	// only be the user's currency, which comes from the request.
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
	// The card comes from the request: an invalid or expired one is the
	// caller's error, with the payment service's reason as the message.
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
