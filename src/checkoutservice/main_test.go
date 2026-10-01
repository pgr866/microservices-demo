package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/checkoutservice/genproto"
)

// Fake clients: each embeds the generated interface so only the RPCs that
// checkoutservice calls need an implementation.

type fakeCart struct {
	pb.CartServiceClient
	items    []*pb.CartItem
	err      error
	emptyErr error
	emptied  string
	gotUser  string
}

func (f *fakeCart) GetCart(_ context.Context, req *pb.GetCartRequest, _ ...grpc.CallOption) (*pb.Cart, error) {
	f.gotUser = req.GetUserId()
	return &pb.Cart{Items: f.items}, f.err
}

func (f *fakeCart) EmptyCart(_ context.Context, req *pb.EmptyCartRequest, _ ...grpc.CallOption) (*pb.Empty, error) {
	f.emptied = req.GetUserId()
	return &pb.Empty{}, f.emptyErr
}

type fakeCatalog struct {
	pb.ProductCatalogServiceClient
	prices map[string]*pb.Money
	err    error
}

func (f *fakeCatalog) GetProduct(_ context.Context, req *pb.GetProductRequest, _ ...grpc.CallOption) (*pb.Product, error) {
	if f.err != nil {
		return nil, f.err
	}
	price, ok := f.prices[req.GetId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "no product with ID "+req.GetId())
	}
	return &pb.Product{Id: req.GetId(), PriceUsd: price}, nil
}

// fakeCurrency doubles the amount, so tests can tell converted and unconverted
// prices apart.
type fakeCurrency struct {
	pb.CurrencyServiceClient
	err        error
	failOnCall int       // if set, only that call (1-based) returns err
	result     *pb.Money // returned as is instead of the doubled amount, if set
	calls      int
}

func (f *fakeCurrency) Convert(_ context.Context, req *pb.CurrencyConversionRequest, _ ...grpc.CallOption) (*pb.Money, error) {
	f.calls++
	if f.err != nil && (f.failOnCall == 0 || f.calls == f.failOnCall) {
		return nil, f.err
	}
	if f.result != nil {
		return f.result, nil
	}
	from := req.GetFrom()
	return &pb.Money{CurrencyCode: req.GetToCode(), Units: from.GetUnits() * 2, Nanos: from.GetNanos() * 2}, nil
}

type fakeShipping struct {
	pb.ShippingServiceClient
	quote    *pb.Money
	quoteErr error
	shipErr  error
	shipped  bool
	shipReq  *pb.ShipOrderRequest
}

func (f *fakeShipping) GetQuote(_ context.Context, _ *pb.GetQuoteRequest, _ ...grpc.CallOption) (*pb.GetQuoteResponse, error) {
	return &pb.GetQuoteResponse{CostUsd: f.quote}, f.quoteErr
}

func (f *fakeShipping) ShipOrder(_ context.Context, req *pb.ShipOrderRequest, _ ...grpc.CallOption) (*pb.ShipOrderResponse, error) {
	f.shipped = true
	f.shipReq = req
	if f.shipErr != nil {
		return nil, f.shipErr
	}
	return &pb.ShipOrderResponse{TrackingId: "TRACK-1"}, nil
}

type fakePayment struct {
	pb.PaymentServiceClient
	err     error
	charged *pb.Money
	card    *pb.CreditCardInfo
}

func (f *fakePayment) Charge(_ context.Context, req *pb.ChargeRequest, _ ...grpc.CallOption) (*pb.ChargeResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.charged = req.GetAmount()
	f.card = req.GetCreditCard()
	return &pb.ChargeResponse{TransactionId: "TX-1"}, nil
}

type fakeEmail struct {
	pb.EmailServiceClient
	err  error
	sent *pb.SendOrderConfirmationRequest
}

func (f *fakeEmail) SendOrderConfirmation(_ context.Context, req *pb.SendOrderConfirmationRequest, _ ...grpc.CallOption) (*pb.Empty, error) {
	f.sent = req
	return &pb.Empty{}, f.err
}

type fakes struct {
	cart     *fakeCart
	catalog  *fakeCatalog
	currency *fakeCurrency
	shipping *fakeShipping
	payment  *fakePayment
	email    *fakeEmail
}

// newTestService returns a service whose cart holds 2 x A (1.25 USD) and
// 1 x B (10 USD), with a shipping quote of 5.25 USD.
func newTestService() (*checkoutService, *fakes) {
	f := &fakes{
		cart: &fakeCart{items: []*pb.CartItem{
			{ProductId: "A", Quantity: 2},
			{ProductId: "B", Quantity: 1},
		}},
		catalog: &fakeCatalog{prices: map[string]*pb.Money{
			"A": {CurrencyCode: "USD", Units: 1, Nanos: 250000000},
			"B": {CurrencyCode: "USD", Units: 10},
		}},
		currency: &fakeCurrency{},
		shipping: &fakeShipping{quote: &pb.Money{CurrencyCode: "USD", Units: 5, Nanos: 250000000}},
		payment:  &fakePayment{},
		email:    &fakeEmail{},
	}
	svc := &checkoutService{
		cartSvc:           f.cart,
		productCatalogSvc: f.catalog,
		currencySvc:       f.currency,
		shippingSvc:       f.shipping,
		paymentSvc:        f.payment,
		emailSvc:          f.email,
	}
	return svc, f
}

func placeOrderRequest() *pb.PlaceOrderRequest {
	return &pb.PlaceOrderRequest{
		UserId:       "user-1",
		UserCurrency: "EUR",
		Email:        "someone@example.com",
		Address:      &pb.Address{City: "Madrid"},
		CreditCard:   &pb.CreditCardInfo{CreditCardNumber: "4432801561520454"},
	}
}

func TestPlaceOrder(t *testing.T) {
	svc, f := newTestService()

	resp, err := svc.PlaceOrder(context.Background(), placeOrderRequest())
	if err != nil {
		t.Fatalf("PlaceOrder failed: %v", err)
	}
	order := resp.GetOrder()

	// Prices and shipping come back doubled from the fake currency service:
	// 2 x 2.50 + 1 x 20 + 10.50 = 35.50 EUR.
	wantTotal := &pb.Money{CurrencyCode: "EUR", Units: 35, Nanos: 500000000}
	if !proto.Equal(f.payment.charged, wantTotal) {
		t.Errorf("charged %v, want %v", f.payment.charged, wantTotal)
	}
	wantShipping := &pb.Money{CurrencyCode: "EUR", Units: 10, Nanos: 500000000}
	if !proto.Equal(order.GetShippingCost(), wantShipping) {
		t.Errorf("shipping cost = %v, want %v", order.GetShippingCost(), wantShipping)
	}
	if len(order.GetItems()) != 2 {
		t.Fatalf("got %d order items, want 2", len(order.GetItems()))
	}
	wantItemCost := &pb.Money{CurrencyCode: "EUR", Units: 2, Nanos: 500000000}
	if !proto.Equal(order.GetItems()[0].GetCost(), wantItemCost) {
		t.Errorf("item A cost = %v, want %v (unit price, not multiplied by quantity)", order.GetItems()[0].GetCost(), wantItemCost)
	}
	if _, err := uuid.Parse(order.GetOrderId()); err != nil {
		t.Errorf("order ID %q is not a valid UUID: %v", order.GetOrderId(), err)
	}
	if order.GetShippingTrackingId() != "TRACK-1" {
		t.Errorf("tracking ID = %q, want TRACK-1", order.GetShippingTrackingId())
	}
	if order.GetShippingAddress().GetCity() != "Madrid" {
		t.Errorf("shipping address = %v, want the one from the request", order.GetShippingAddress())
	}
	if f.cart.gotUser != "user-1" {
		t.Errorf("read cart of %q, want user-1", f.cart.gotUser)
	}
	if f.payment.card.GetCreditCardNumber() != "4432801561520454" {
		t.Errorf("charged card %v, want the one from the request", f.payment.card)
	}
	if f.shipping.shipReq.GetAddress().GetCity() != "Madrid" || len(f.shipping.shipReq.GetItems()) != 2 {
		t.Errorf("ShipOrder request = %v, want the request's address and the 2 cart items", f.shipping.shipReq)
	}
	if f.cart.emptied != "user-1" {
		t.Errorf("emptied cart of %q, want user-1", f.cart.emptied)
	}
	if f.email.sent.GetEmail() != "someone@example.com" || f.email.sent.GetOrder() != order {
		t.Errorf("confirmation email = %v, want the placed order sent to someone@example.com", f.email.sent)
	}
}

func TestPlaceOrder_failures(t *testing.T) {
	boom := errors.New("boom") // what a client returns for a non-gRPC failure
	unavailable := status.Error(codes.Unavailable, "connection refused")
	invalid := status.Error(codes.InvalidArgument, "bad input")
	tests := []struct {
		name  string
		setup func(*fakes)
		req   func(*pb.PlaceOrderRequest)
		// afterPayment: the order fails once the card is charged, so both the
		// charge and the shipment were attempted.
		afterPayment bool
		wantCode     codes.Code
	}{
		// Invalid request, rejected before calling any service.
		{name: "missing user_id", req: func(r *pb.PlaceOrderRequest) { r.UserId = "" }, wantCode: codes.InvalidArgument},
		{name: "missing user_currency", req: func(r *pb.PlaceOrderRequest) { r.UserCurrency = "" }, wantCode: codes.InvalidArgument},
		{name: "missing address", req: func(r *pb.PlaceOrderRequest) { r.Address = nil }, wantCode: codes.InvalidArgument},
		{name: "missing credit_card", req: func(r *pb.PlaceOrderRequest) { r.CreditCard = nil }, wantCode: codes.InvalidArgument},
		// Rejected by a dependency, but the request is at fault.
		{name: "unsupported user_currency", setup: func(f *fakes) { f.currency.err = invalid }, wantCode: codes.InvalidArgument},
		{name: "card declined", setup: func(f *fakes) { f.payment.err = invalid }, wantCode: codes.InvalidArgument},
		// The cart can't be checked out as it is.
		{name: "empty cart", setup: func(f *fakes) { f.cart.items = nil }, wantCode: codes.FailedPrecondition},
		{name: "unknown product", setup: func(f *fakes) { f.cart.items = []*pb.CartItem{{ProductId: "missing", Quantity: 1}} }, wantCode: codes.FailedPrecondition},
		{name: "zero quantity", setup: func(f *fakes) { f.cart.items = []*pb.CartItem{{ProductId: "A", Quantity: 0}} }, wantCode: codes.FailedPrecondition},
		{name: "negative quantity", setup: func(f *fakes) { f.cart.items = []*pb.CartItem{{ProductId: "A", Quantity: -1}} }, wantCode: codes.FailedPrecondition},
		// A dependency is down: worth retrying.
		{name: "cart unavailable", setup: func(f *fakes) { f.cart.err = unavailable }, wantCode: codes.Unavailable},
		{name: "catalog unavailable", setup: func(f *fakes) { f.catalog.err = unavailable }, wantCode: codes.Unavailable},
		{name: "currency unavailable", setup: func(f *fakes) { f.currency.err = unavailable }, wantCode: codes.Unavailable},
		{name: "payment unavailable", setup: func(f *fakes) { f.payment.err = unavailable }, wantCode: codes.Unavailable},
		// The dependency timed out or is overloaded, but the caller's deadline has not passed.
		{name: "payment times out", setup: func(f *fakes) { f.payment.err = status.Error(codes.DeadlineExceeded, "timeout") }, wantCode: codes.Unavailable},
		{name: "payment overloaded", setup: func(f *fakes) { f.payment.err = status.Error(codes.ResourceExhausted, "too many requests") }, wantCode: codes.Unavailable},
		{name: "shipping unavailable", setup: func(f *fakes) { f.shipping.shipErr = unavailable }, afterPayment: true, wantCode: codes.Unavailable},
		// Anything else is a failure retrying won't fix.
		{name: "cart fails", setup: func(f *fakes) { f.cart.err = boom }, wantCode: codes.Internal},
		{name: "currency conversion fails", setup: func(f *fakes) { f.currency.err = boom }, wantCode: codes.Internal},
		{name: "shipping quote fails", setup: func(f *fakes) { f.shipping.quoteErr = boom }, wantCode: codes.Internal},
		// Calls 1 and 2 convert the prices of the two cart items; call 3, the shipping cost.
		{name: "shipping cost conversion fails", setup: func(f *fakes) { f.currency.err = boom; f.currency.failOnCall = 3 }, wantCode: codes.Internal},
		{name: "payment fails", setup: func(f *fakes) { f.payment.err = boom }, wantCode: codes.Internal},
		// A has quantity 2, so MultiplySlow rejects the price; B has quantity 1,
		// so MultiplySlow returns it as is and the sum rejects it.
		{name: "invalid product price (quantity 2)", setup: func(f *fakes) { f.catalog.prices["A"] = &pb.Money{CurrencyCode: "USD", Units: 1, Nanos: -1} }, wantCode: codes.Internal},
		{name: "invalid product price (quantity 1)", setup: func(f *fakes) { f.catalog.prices["B"] = &pb.Money{CurrencyCode: "USD", Units: 1, Nanos: -1} }, wantCode: codes.Internal},
		{name: "invalid converted amount", setup: func(f *fakes) { f.currency.result = &pb.Money{CurrencyCode: "EUR", Units: 1, Nanos: -1} }, wantCode: codes.Internal},
		{name: "converted amount in another currency", setup: func(f *fakes) { f.currency.result = &pb.Money{CurrencyCode: "JPY", Units: 1} }, wantCode: codes.Internal},
		{name: "shipping fails", setup: func(f *fakes) { f.shipping.shipErr = boom }, afterPayment: true, wantCode: codes.Internal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, f := newTestService()
			if tt.setup != nil {
				tt.setup(f)
			}
			req := placeOrderRequest()
			if tt.req != nil {
				tt.req(req)
			}

			_, err := svc.PlaceOrder(context.Background(), req)
			if got := status.Code(err); got != tt.wantCode {
				t.Errorf("status code = %v, want %v (err: %v)", got, tt.wantCode, err)
			}
			if tt.req != nil && f.cart.gotUser != "" {
				t.Error("an invalid request must be rejected before calling any service")
			}
			if (f.payment.charged != nil) != tt.afterPayment {
				t.Errorf("charged = %v, want a charge only if the order fails after the payment", f.payment.charged)
			}
			if f.shipping.shipped != tt.afterPayment {
				t.Errorf("shipped = %v, want %v", f.shipping.shipped, tt.afterPayment)
			}
			if f.cart.emptied != "" || f.email.sent != nil {
				t.Error("a failed order must not empty the cart or send a confirmation email")
			}
		})
	}
}

func TestPlaceOrder_callerGivesUp(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelExpired()

	tests := []struct {
		name     string
		ctx      context.Context
		cartErr  error // what the cart client returns for that context
		wantCode codes.Code
	}{
		{"canceled", canceled, status.Error(codes.Canceled, "context canceled"), codes.Canceled},
		{"deadline exceeded", expired, status.Error(codes.DeadlineExceeded, "context deadline exceeded"), codes.DeadlineExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, f := newTestService()
			f.cart.err = tt.cartErr

			_, err := svc.PlaceOrder(tt.ctx, placeOrderRequest())
			if got := status.Code(err); got != tt.wantCode {
				t.Errorf("status code = %v, want %v (err: %v)", got, tt.wantCode, err)
			}
		})
	}
}

func TestPlaceOrder_declinedCardKeepsPaymentReason(t *testing.T) {
	svc, f := newTestService()
	f.payment.err = status.Error(codes.InvalidArgument, "Credit card info is invalid")

	_, err := svc.PlaceOrder(context.Background(), placeOrderRequest())
	if got := status.Convert(err).Message(); got != "Credit card info is invalid" {
		t.Errorf("message = %q, want the payment service's reason", got)
	}
}

func TestPlaceOrder_sideEffectFailuresDoNotFailOrder(t *testing.T) {
	svc, f := newTestService()
	f.cart.emptyErr = errors.New("cart unavailable")
	f.email.err = errors.New("email unavailable")

	resp, err := svc.PlaceOrder(context.Background(), placeOrderRequest())
	if err != nil {
		t.Fatalf("PlaceOrder failed: %v", err)
	}
	if resp.GetOrder().GetShippingTrackingId() != "TRACK-1" {
		t.Errorf("tracking ID = %q, want TRACK-1", resp.GetOrder().GetShippingTrackingId())
	}
}

func TestMustConnGRPC(t *testing.T) {
	t.Setenv("TEST_SERVICE_ADDR", "localhost:1")

	// The connection is lazy, so nothing needs to listen on that address.
	conn := mustConnGRPC("TEST_SERVICE_ADDR")
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
	})
	if conn.Target() != "localhost:1" {
		t.Errorf("target = %q, want localhost:1", conn.Target())
	}
}

func TestMustConnGRPC_missingEnvExits(t *testing.T) {
	t.Setenv("TEST_SERVICE_ADDR", "")
	exitCode := 0
	log.ExitFunc = func(code int) {
		exitCode = code
		// Stop here as os.Exit would, instead of carrying on to grpc.NewClient.
		panic("exit")
	}
	defer func() { log.ExitFunc = nil }()

	func() {
		defer func() { _ = recover() }()
		mustConnGRPC("TEST_SERVICE_ADDR")
	}()
	if exitCode != 1 {
		t.Errorf("exit code = %d, want 1 when the address variable is not set", exitCode)
	}
}

func TestPlaceOrder_doesNotLogPersonalData(t *testing.T) {
	var out bytes.Buffer
	log.Out = &out
	level := log.Level
	log.Level = logrus.DebugLevel
	t.Cleanup(func() { log.Out, log.Level = os.Stdout, level })
	for _, emailErr := range []error{nil, errors.New("email unavailable")} {
		svc, f := newTestService()
		f.email.err = emailErr

		if _, err := svc.PlaceOrder(context.Background(), placeOrderRequest()); err != nil {
			t.Fatal(err)
		}
	}
	// The email address, and the user ID, which is the session ID that keys the cart.
	for _, personal := range []string{"someone@example.com", "user-1"} {
		if strings.Contains(out.String(), personal) {
			t.Errorf("%q reached the logs:\n%s", personal, out.String())
		}
	}
}
