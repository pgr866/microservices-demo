package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/frontend/genproto"
)

// Fakes embed the generated interface so only the RPCs frontend calls need implementing.

type fakeCatalog struct {
	pb.ProductCatalogServiceClient
	products []*pb.Product
	err      error                    // returned by ListProducts
	delays   map[string]time.Duration // per product, to make GetProduct calls finish out of order
	hangID   string                   // if set, GetProduct of that product never answers until cancelled
}

func (f *fakeCatalog) ListProducts(context.Context, *pb.Empty, ...grpc.CallOption) (*pb.ListProductsResponse, error) {
	return &pb.ListProductsResponse{Products: f.products}, f.err
}

func (f *fakeCatalog) GetProduct(ctx context.Context, req *pb.GetProductRequest, _ ...grpc.CallOption) (*pb.Product, error) {
	if req.GetId() == f.hangID {
		<-ctx.Done()
		return nil, status.FromContextError(ctx.Err()).Err()
	}
	time.Sleep(f.delays[req.GetId()])
	for _, p := range f.products {
		if p.GetId() == req.GetId() {
			return p, nil
		}
	}
	return nil, status.Error(codes.NotFound, "no product with ID "+req.GetId())
}

// fakeCurrency doubles the amount, so converted prices are distinguishable.
type fakeCurrency struct {
	pb.CurrencyServiceClient
	codes      []string
	listErr    error // returned by GetSupportedCurrencies
	convertErr error
	failAfter  int        // with convertErr, number of conversions that succeed first
	mu         sync.Mutex // pages convert the prices of several products concurrently
	converted  int
}

func (f *fakeCurrency) GetSupportedCurrencies(context.Context, *pb.Empty, ...grpc.CallOption) (*pb.GetSupportedCurrenciesResponse, error) {
	return &pb.GetSupportedCurrenciesResponse{CurrencyCodes: f.codes}, f.listErr
}

func (f *fakeCurrency) Convert(_ context.Context, req *pb.CurrencyConversionRequest, _ ...grpc.CallOption) (*pb.Money, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.convertErr != nil && f.converted >= f.failAfter {
		return nil, f.convertErr
	}
	f.converted++
	from := req.GetFrom()
	return &pb.Money{CurrencyCode: req.GetToCode(), Units: from.GetUnits() * 2, Nanos: from.GetNanos() * 2}, nil
}

type fakeCart struct {
	pb.CartServiceClient
	items   []*pb.CartItem
	err     error // returned by every RPC
	added   *pb.AddItemRequest
	emptied string
}

func (f *fakeCart) GetCart(context.Context, *pb.GetCartRequest, ...grpc.CallOption) (*pb.Cart, error) {
	return &pb.Cart{Items: f.items}, f.err
}

func (f *fakeCart) AddItem(_ context.Context, req *pb.AddItemRequest, _ ...grpc.CallOption) (*pb.Empty, error) {
	if f.err == nil {
		f.added = req
	}
	return &pb.Empty{}, f.err
}

func (f *fakeCart) EmptyCart(_ context.Context, req *pb.EmptyCartRequest, _ ...grpc.CallOption) (*pb.Empty, error) {
	if f.err == nil {
		f.emptied = req.GetUserId()
	}
	return &pb.Empty{}, f.err
}

type fakeRecommendations struct {
	pb.RecommendationServiceClient
	ids []string
	err error
}

func (f *fakeRecommendations) ListRecommendations(context.Context, *pb.ListRecommendationsRequest, ...grpc.CallOption) (*pb.ListRecommendationsResponse, error) {
	return &pb.ListRecommendationsResponse{ProductIds: f.ids}, f.err
}

type fakeShipping struct {
	pb.ShippingServiceClient
	quote *pb.Money
	err   error
}

func (f *fakeShipping) GetQuote(context.Context, *pb.GetQuoteRequest, ...grpc.CallOption) (*pb.GetQuoteResponse, error) {
	return &pb.GetQuoteResponse{CostUsd: f.quote}, f.err
}

type fakeCheckout struct {
	pb.CheckoutServiceClient
	got   *pb.PlaceOrderRequest
	order *pb.OrderResult
	err   error
}

func (f *fakeCheckout) PlaceOrder(_ context.Context, req *pb.PlaceOrderRequest, _ ...grpc.CallOption) (*pb.PlaceOrderResponse, error) {
	f.got = req
	if f.err != nil {
		return nil, f.err
	}
	return &pb.PlaceOrderResponse{Order: f.order}, nil
}

type fakeAds struct {
	pb.AdServiceClient
	ads   []*pb.Ad
	err   error
	got   *pb.AdRequest
	calls int
}

func (f *fakeAds) GetAds(_ context.Context, req *pb.AdRequest, _ ...grpc.CallOption) (*pb.AdResponse, error) {
	f.calls++
	f.got = req
	return &pb.AdResponse{Ads: f.ads}, f.err
}

type fakes struct {
	catalog         *fakeCatalog
	currency        *fakeCurrency
	cart            *fakeCart
	recommendations *fakeRecommendations
	shipping        *fakeShipping
	checkout        *fakeCheckout
	ads             *fakeAds
}

// newTestFrontend: catalog A (1.25 USD), B (10 USD); cart 2 x A + 1 x B; shipping 5.25 USD.
func newTestFrontend() (*frontendServer, *fakes) {
	f := &fakes{
		catalog: &fakeCatalog{products: []*pb.Product{
			{Id: "A", Name: "Product A", PriceUsd: &pb.Money{CurrencyCode: "USD", Units: 1, Nanos: 250000000}},
			{Id: "B", Name: "Product B", PriceUsd: &pb.Money{CurrencyCode: "USD", Units: 10}},
		}},
		currency: &fakeCurrency{codes: []string{"EUR", "USD", "XYZ"}},
		cart: &fakeCart{items: []*pb.CartItem{
			{ProductId: "A", Quantity: 2},
			{ProductId: "B", Quantity: 1},
		}},
		recommendations: &fakeRecommendations{},
		shipping:        &fakeShipping{quote: &pb.Money{CurrencyCode: "USD", Units: 5, Nanos: 250000000}},
		checkout:        &fakeCheckout{},
		ads:             &fakeAds{},
	}
	svc := &frontendServer{
		productCatalogSvc: f.catalog,
		currencySvc:       f.currency,
		cartSvc:           f.cart,
		recommendationSvc: f.recommendations,
		checkoutSvc:       f.checkout,
		shippingSvc:       f.shipping,
		adSvc:             f.ads,
	}
	return svc, f
}

// serve runs r through main's handler chain as user "user-1" with EUR.
func serve(svc *frontendServer, r *http.Request) *httptest.ResponseRecorder {
	logger := logrus.New()
	logger.Out = io.Discard
	r.AddCookie(&http.Cookie{Name: cookieSessionID, Value: "user-1"})
	r.AddCookie(&http.Cookie{Name: cookieCurrency, Value: "EUR"})
	w := httptest.NewRecorder()
	newHandler(svc, logger, time.Minute).ServeHTTP(w, r)
	return w
}

func postForm(target string, form url.Values) *http.Request {
	r := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func assertContains(t *testing.T, body string, want ...string) {
	t.Helper()
	for _, s := range want {
		if !strings.Contains(body, s) {
			t.Errorf("page does not contain %q", s)
		}
	}
}

func TestHome(t *testing.T) {
	svc, _ := newTestFrontend()

	w := serve(svc, httptest.NewRequest(http.MethodGet, "/", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	// Prices come back doubled from the fake currency service.
	assertContains(t, w.Body.String(), "Product A", "€2.50", "Product B", "€20.00")
	if strings.Contains(w.Body.String(), "XYZ") {
		t.Error("page offers XYZ, a currency outside the whitelist")
	}
}

func TestHome_doesNotCallAdservice(t *testing.T) {
	svc, f := newTestFrontend()

	// The home page has no ad slot: an ad call would add latency to the most-loaded page.
	w := serve(svc, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusOK || f.ads.calls != 0 {
		t.Errorf("status = %d, adservice calls = %d, want 200 and none", w.Code, f.ads.calls)
	}
}

func TestNewSessionCookie(t *testing.T) {
	svc, _ := newTestFrontend()
	logger := logrus.New()
	logger.Out = io.Discard

	// Also when the first page is a nested one: a cookie scoped to /product would not reach /cart.
	for _, target := range []string{"/", "/product/A"} {
		w := httptest.NewRecorder()
		// No session cookie in the request, unlike serve.
		newHandler(svc, logger, time.Minute).ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))

		cookies := w.Result().Cookies()
		if len(cookies) != 1 || cookies[0].Name != cookieSessionID || cookies[0].Value == "" {
			t.Fatalf("%s: cookies = %v, want a new session cookie", target, cookies)
		}
		if !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].Path != "/" {
			t.Errorf("%s: session cookie is not HttpOnly, SameSite=Lax and for the whole site: %v", target, cookies[0])
		}
	}
}

func TestProduct(t *testing.T) {
	svc, _ := newTestFrontend()

	w := serve(svc, httptest.NewRequest(http.MethodGet, "/product/A", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	assertContains(t, w.Body.String(), "Product A", "€2.50")

	if w := serve(svc, httptest.NewRequest(http.MethodGet, "/product/missing", nil)); w.Code != http.StatusNotFound {
		t.Errorf("unknown product: status = %d, want 404", w.Code)
	}
}

func TestProduct_showsAnAdForItsCategories(t *testing.T) {
	svc, f := newTestFrontend()
	f.catalog.products[0].Categories = []string{"kitchen"}
	f.ads.ads = []*pb.Ad{{Text: "Mugs on sale", RedirectUrl: "/product/A"}}

	w := serve(svc, httptest.NewRequest(http.MethodGet, "/product/A", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	assertContains(t, w.Body.String(), "Mugs on sale")
	if got := f.ads.got.GetContextKeys(); len(got) != 1 || got[0] != "kitchen" {
		t.Errorf("ad context keys = %v, want the product's categories", got)
	}
}

func TestProduct_failedAdIsNotCritical(t *testing.T) {
	svc, f := newTestFrontend()
	f.ads.err = status.Error(codes.Unavailable, "down")

	if w := serve(svc, httptest.NewRequest(http.MethodGet, "/product/A", nil)); w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 without the ad", w.Code)
	}
}

func TestProduct_failedRecommendationsAreNotCritical(t *testing.T) {
	svc, f := newTestFrontend()
	f.recommendations.err = status.Error(codes.Unavailable, "down")

	if w := serve(svc, httptest.NewRequest(http.MethodGet, "/product/A", nil)); w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 without recommendations", w.Code)
	}
}

func TestAddToCart(t *testing.T) {
	svc, f := newTestFrontend()

	w := serve(svc, postForm("/cart", url.Values{"product_id": {"A"}, "quantity": {"2"}}))

	if w.Code != http.StatusFound || w.Header().Get("Location") != "/cart" {
		t.Errorf("response = %d to %q, want a redirect to /cart", w.Code, w.Header().Get("Location"))
	}
	got := f.cart.added
	if got.GetUserId() != "user-1" || got.GetItem().GetProductId() != "A" || got.GetItem().GetQuantity() != 2 {
		t.Errorf("AddItem request = %v, want 2 x A for user-1", got)
	}
}

func TestAddToCart_rejected(t *testing.T) {
	tests := []struct {
		name     string
		form     url.Values
		wantCode int
	}{
		{"quantity 0", url.Values{"product_id": {"A"}, "quantity": {"0"}}, http.StatusUnprocessableEntity},
		{"quantity over 10", url.Values{"product_id": {"A"}, "quantity": {"11"}}, http.StatusUnprocessableEntity},
		{"no product", url.Values{"quantity": {"1"}}, http.StatusUnprocessableEntity},
		{"unknown product", url.Values{"product_id": {"missing"}, "quantity": {"1"}}, http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, f := newTestFrontend()

			if w := serve(svc, postForm("/cart", tt.form)); w.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", w.Code, tt.wantCode)
			}
			if f.cart.added != nil {
				t.Error("a rejected item must not reach the cart")
			}
		})
	}
}

func TestViewCart(t *testing.T) {
	svc, _ := newTestFrontend()

	w := serve(svc, httptest.NewRequest(http.MethodGet, "/cart", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	// 2 x 2.50 + 1 x 20 + 10.50 shipping = 35.50 EUR.
	assertContains(t, w.Body.String(), "€5.00", "€20.00", "€10.50", "€35.50")
}

func TestViewCart_productNoLongerInCatalog(t *testing.T) {
	svc, f := newTestFrontend()
	f.cart.items = append(f.cart.items, &pb.CartItem{ProductId: "gone", Quantity: 1})

	// The catalog's NotFound is passed on, instead of a generic 500.
	if w := serve(svc, httptest.NewRequest(http.MethodGet, "/cart", nil)); w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestViewCart_invalidPriceIsAnError(t *testing.T) {
	svc, f := newTestFrontend()
	f.catalog.products[0].PriceUsd = &pb.Money{CurrencyCode: "USD", Units: 1, Nanos: -1}

	// An invalid amount must get an error page, not a panic.
	if w := serve(svc, httptest.NewRequest(http.MethodGet, "/cart", nil)); w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestViewCart_failedRecommendationsAreNotCritical(t *testing.T) {
	svc, f := newTestFrontend()
	f.recommendations.err = status.Error(codes.Unavailable, "down")

	w := serve(svc, httptest.NewRequest(http.MethodGet, "/cart", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 without recommendations", w.Code)
	}
	assertContains(t, w.Body.String(), "€35.50")
}

func TestViewCart_itemConversionDown(t *testing.T) {
	svc, f := newTestFrontend()
	// The shipping cost converts; the first cart item does not.
	f.currency.convertErr = status.Error(codes.Unavailable, "down")
	f.currency.failAfter = 1

	if w := serve(svc, httptest.NewRequest(http.MethodGet, "/cart", nil)); w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

func TestConvertPrices_keepsProductOrder(t *testing.T) {
	svc, f := newTestFrontend()
	// The first product answers last, so completion order is the reverse.
	products := f.catalog.products
	slowFirst := &slowCurrency{fakeCurrency: f.currency, slow: products[0].GetPriceUsd()}
	svc.currencySvc = slowFirst

	prices, err := svc.convertPrices(context.Background(), products, "EUR")
	if err != nil {
		t.Fatalf("convertPrices: %v", err)
	}
	if len(prices) != 2 || prices[0].GetUnits() != 2 || prices[1].GetUnits() != 20 {
		t.Errorf("prices = %v, want A's then B's", prices)
	}
}

func TestGetCartProducts_keepsCartOrder(t *testing.T) {
	svc, f := newTestFrontend()
	f.catalog.delays = map[string]time.Duration{"A": 50 * time.Millisecond}

	products, prices, err := svc.getCartProducts(context.Background(), f.cart.items, "EUR")
	if err != nil {
		t.Fatalf("getCartProducts: %v", err)
	}
	if len(products) != 2 || products[0].GetId() != "A" || products[1].GetId() != "B" {
		t.Errorf("products = %v, want the cart order A, B", products)
	}
	if len(prices) != 2 || prices[0].GetUnits() != 2 || prices[1].GetUnits() != 20 {
		t.Errorf("prices = %v, want A's then B's", prices)
	}
}

func TestGetCartProducts_firstErrorCancelsTheRest(t *testing.T) {
	svc, f := newTestFrontend()
	// A would hang forever unless the missing product's error cancels it.
	f.catalog.hangID = "A"
	cart := []*pb.CartItem{{ProductId: "A", Quantity: 1}, {ProductId: "gone", Quantity: 1}}

	_, _, err := svc.getCartProducts(context.Background(), cart, "EUR")
	if status.Code(err) != codes.NotFound || !strings.Contains(err.Error(), "#gone") {
		t.Fatalf("err = %v, want NotFound for product #gone", err)
	}
}

// slowCurrency delays the conversion of one amount.
type slowCurrency struct {
	*fakeCurrency
	slow *pb.Money
}

func (s *slowCurrency) Convert(ctx context.Context, req *pb.CurrencyConversionRequest, opts ...grpc.CallOption) (*pb.Money, error) {
	if req.GetFrom() == s.slow {
		time.Sleep(50 * time.Millisecond)
	}
	return s.fakeCurrency.Convert(ctx, req, opts...)
}

func checkoutForm() url.Values {
	return url.Values{
		"email":                        {"someone@example.com"},
		"street_address":               {"1600 Amphitheatre Parkway"},
		"zip_code":                     {"94043"},
		"city":                         {"Mountain View"},
		"state":                        {"CA"},
		"country":                      {"USA"},
		"credit_card_number":           {"4432801561520454"},
		"credit_card_expiration_month": {"1"},
		"credit_card_expiration_year":  {"2030"},
		"credit_card_cvv":              {"672"},
	}
}

func TestPlaceOrder(t *testing.T) {
	svc, f := newTestFrontend()
	f.checkout.order = &pb.OrderResult{
		OrderId:      "order-1",
		ShippingCost: &pb.Money{CurrencyCode: "EUR", Units: 10, Nanos: 500000000},
		Items: []*pb.OrderItem{
			{Item: &pb.CartItem{ProductId: "A", Quantity: 2}, Cost: &pb.Money{CurrencyCode: "EUR", Units: 2, Nanos: 500000000}},
			{Item: &pb.CartItem{ProductId: "B", Quantity: 1}, Cost: &pb.Money{CurrencyCode: "EUR", Units: 20}},
		},
	}

	w := serve(svc, postForm("/cart/checkout", checkoutForm()))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	assertContains(t, w.Body.String(), "order-1", "€35.50")
	got := f.checkout.got
	if got.GetUserId() != "user-1" || got.GetUserCurrency() != "EUR" || got.GetEmail() != "someone@example.com" {
		t.Errorf("PlaceOrder request = %v, want user-1 paying in EUR", got)
	}
	if got.GetCreditCard().GetCreditCardNumber() != "4432801561520454" || got.GetCreditCard().GetCreditCardExpirationYear() != 2030 {
		t.Errorf("credit card = %v, want the one from the form", got.GetCreditCard())
	}
	if got.GetAddress().GetZipCode() != 94043 || got.GetAddress().GetCity() != "Mountain View" {
		t.Errorf("address = %v, want the one from the form", got.GetAddress())
	}
}

func TestPlaceOrder_rejected(t *testing.T) {
	invalidEmail := checkoutForm()
	invalidEmail.Set("email", "not-an-email")
	invalidMonth := checkoutForm()
	invalidMonth.Set("credit_card_expiration_month", "13")

	tests := []struct {
		name     string
		form     url.Values
		err      error // returned by checkoutservice
		wantCode int
	}{
		{"invalid email", invalidEmail, nil, http.StatusUnprocessableEntity},
		{"invalid expiration month", invalidMonth, nil, http.StatusUnprocessableEntity},
		{"card declined", checkoutForm(), status.Error(codes.InvalidArgument, "Your credit card expired"), http.StatusBadRequest},
		{"empty cart", checkoutForm(), status.Error(codes.FailedPrecondition, "cart is empty"), http.StatusBadRequest},
		{"checkout down", checkoutForm(), status.Error(codes.Unavailable, "down"), http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, f := newTestFrontend()
			f.checkout.err = tt.err

			if w := serve(svc, postForm("/cart/checkout", tt.form)); w.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", w.Code, tt.wantCode)
			}
			if tt.wantCode == http.StatusUnprocessableEntity && f.checkout.got != nil {
				t.Error("an invalid form must not reach checkoutservice")
			}
		})
	}
}

func TestEmptyCart(t *testing.T) {
	svc, f := newTestFrontend()

	w := serve(svc, postForm("/cart/empty", nil))

	if w.Code != http.StatusFound || w.Header().Get("Location") != "/" {
		t.Errorf("response = %d to %q, want a redirect to /", w.Code, w.Header().Get("Location"))
	}
	if f.cart.emptied != "user-1" {
		t.Errorf("emptied cart of %q, want user-1", f.cart.emptied)
	}
}

func TestSetCurrency(t *testing.T) {
	svc, _ := newTestFrontend()
	r := postForm("/setCurrency", url.Values{"currency_code": {"JPY"}})
	r.Header.Set("Referer", "http://"+r.Host+"/product/A")

	w := serve(svc, r)

	if w.Code != http.StatusFound || w.Header().Get("Location") != "/product/A" {
		t.Errorf("response = %d to %q, want a redirect back to /product/A", w.Code, w.Header().Get("Location"))
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != cookieCurrency || cookies[0].Value != "JPY" {
		t.Fatalf("cookies = %v, want the currency cookie set to JPY", cookies)
	}
	if !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].Path != "/" {
		t.Errorf("currency cookie is not HttpOnly, SameSite=Lax and for the whole site: %v", cookies[0])
	}

	if w := serve(svc, postForm("/setCurrency", url.Values{"currency_code": {"NOPE"}})); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("invalid currency: status = %d, want 422", w.Code)
	}
}

func TestSameSiteReferer(t *testing.T) {
	tests := []struct {
		referer string
		want    string
	}{
		{"http://shop.example/cart?x=1", "/cart?x=1"},
		{"", "/"},
		{"http://evil.example/cart", "/"},
		{"http://shop.example//evil.example/", "/"},
		// Escaped, so the browser keeps it as a path of this site.
		{"http://shop.example/\\evil.example/", "/%5Cevil.example/"},
		{"::not a url", "/"},
	}
	for _, tt := range tests {
		t.Run(tt.referer, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "http://shop.example/setCurrency", nil)
			r.Header.Set("Referer", tt.referer)
			if got := sameSiteReferer(r); got != tt.want {
				t.Errorf("sameSiteReferer(%q) = %q, want %q", tt.referer, got, tt.want)
			}
		})
	}
}

func TestLogout(t *testing.T) {
	svc, _ := newTestFrontend()

	w := serve(svc, httptest.NewRequest(http.MethodGet, "/logout", nil))

	if w.Code != http.StatusFound || w.Header().Get("Location") != "/" {
		t.Errorf("response = %d to %q, want a redirect to /", w.Code, w.Header().Get("Location"))
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("got %d cookies, want the session and currency ones expired", len(cookies))
	}
	for _, c := range cookies {
		// Same path they were set with, or the browser would keep them.
		if c.MaxAge >= 0 || c.Path != "/" {
			t.Errorf("cookie %s is not expired for the whole site (MaxAge %d, Path %q)", c.Name, c.MaxAge, c.Path)
		}
	}
}

func TestRoutes(t *testing.T) {
	svc, _ := newTestFrontend()
	tests := []struct {
		method, path string
		wantCode     int
		wantBody     string
	}{
		{http.MethodGet, "/_healthz", http.StatusOK, "ok"},
		{http.MethodGet, "/robots.txt", http.StatusOK, "Disallow: /"},
		{http.MethodGet, "/static/styles/styles.css", http.StatusOK, ""},
		{http.MethodHead, "/", http.StatusOK, ""},
		// http.FileServer would list the files in it.
		{http.MethodGet, "/static/styles/", http.StatusNotFound, ""},
		{http.MethodPost, "/", http.StatusMethodNotAllowed, ""},
		{http.MethodGet, "/no-such-page", http.StatusNotFound, ""},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			w := serve(svc, httptest.NewRequest(tt.method, tt.path, nil))
			if w.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", w.Code, tt.wantCode)
			}
			if !strings.Contains(w.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want it to contain %q", w.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestGetRecommendationsTakesFour(t *testing.T) {
	svc, f := newTestFrontend()
	for _, id := range []string{"C", "D", "E", "F", "G"} {
		f.catalog.products = append(f.catalog.products, &pb.Product{Id: id})
	}
	f.recommendations.ids = []string{"A", "B", "C", "D", "E"}

	got, err := svc.getRecommendations(context.Background(), "user-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Errorf("got %d recommendations, want the first 4 to fit the UI", len(got))
	}
}

func TestGetRecommendations_unknownProduct(t *testing.T) {
	svc, f := newTestFrontend()
	f.recommendations.ids = []string{"A", "gone"}

	if got, err := svc.getRecommendations(context.Background(), "user-1", nil); err == nil {
		t.Errorf("got %v, want an error for a recommended product missing from the catalog", got)
	}
	// Recommendations are not critical: the page is still served.
	if w := serve(svc, httptest.NewRequest(http.MethodGet, "/product/A", nil)); w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestChooseAd(t *testing.T) {
	svc, f := newTestFrontend()
	logger := logrus.New()
	logger.Out = io.Discard
	f.ads.ads = []*pb.Ad{{Text: "only ad"}}

	if ad := svc.chooseAd(context.Background(), nil, logger); ad.GetText() != "only ad" {
		t.Errorf("ad = %v, want the only one available", ad)
	}

	f.ads.ads = nil
	if ad := svc.chooseAd(context.Background(), nil, logger); ad != nil {
		t.Errorf("ad = %v, want none when adservice has no ads", ad)
	}

	f.ads.err = status.Error(codes.Unavailable, "down")
	if ad := svc.chooseAd(context.Background(), nil, logger); ad != nil {
		t.Errorf("ad = %v, want none when adservice fails", ad)
	}
}

func TestOrderTotal_invalidAmount(t *testing.T) {
	order := &pb.OrderResult{
		ShippingCost: &pb.Money{CurrencyCode: "EUR", Units: 1},
		Items:        []*pb.OrderItem{{Item: &pb.CartItem{Quantity: 2}, Cost: &pb.Money{CurrencyCode: "EUR", Units: 1, Nanos: -1}}},
	}
	if _, err := orderTotal(order); err == nil {
		t.Error("orderTotal accepted an invalid item cost")
	}
}

func TestOrderTotal_mixedCurrencies(t *testing.T) {
	order := &pb.OrderResult{
		ShippingCost: &pb.Money{CurrencyCode: "EUR", Units: 1},
		Items:        []*pb.OrderItem{{Item: &pb.CartItem{Quantity: 1}, Cost: &pb.Money{CurrencyCode: "USD", Units: 1}}},
	}
	if _, err := orderTotal(order); err == nil {
		t.Error("orderTotal added an item cost in another currency than the shipping cost")
	}
}

func TestDependencyDown(t *testing.T) {
	down := status.Error(codes.Unavailable, "connection refused")
	tests := []struct {
		name  string
		setup func(*fakes)
		req   func() *http.Request
	}{
		{"home: currencies", func(f *fakes) { f.currency.listErr = down }, func() *http.Request { return httptest.NewRequest(http.MethodGet, "/", nil) }},
		{"home: products", func(f *fakes) { f.catalog.err = down }, func() *http.Request { return httptest.NewRequest(http.MethodGet, "/", nil) }},
		{"home: cart", func(f *fakes) { f.cart.err = down }, func() *http.Request { return httptest.NewRequest(http.MethodGet, "/", nil) }},
		{"home: conversion", func(f *fakes) { f.currency.convertErr = down }, func() *http.Request { return httptest.NewRequest(http.MethodGet, "/", nil) }},
		{"product: currencies", func(f *fakes) { f.currency.listErr = down }, func() *http.Request { return httptest.NewRequest(http.MethodGet, "/product/A", nil) }},
		{"product: cart", func(f *fakes) { f.cart.err = down }, func() *http.Request { return httptest.NewRequest(http.MethodGet, "/product/A", nil) }},
		{"product: conversion", func(f *fakes) { f.currency.convertErr = down }, func() *http.Request { return httptest.NewRequest(http.MethodGet, "/product/A", nil) }},
		{"cart: currencies", func(f *fakes) { f.currency.listErr = down }, func() *http.Request { return httptest.NewRequest(http.MethodGet, "/cart", nil) }},
		{"cart: cart", func(f *fakes) { f.cart.err = down }, func() *http.Request { return httptest.NewRequest(http.MethodGet, "/cart", nil) }},
		{"cart: shipping", func(f *fakes) { f.shipping.err = down }, func() *http.Request { return httptest.NewRequest(http.MethodGet, "/cart", nil) }},
		{"cart: conversion", func(f *fakes) { f.currency.convertErr = down }, func() *http.Request { return httptest.NewRequest(http.MethodGet, "/cart", nil) }},
		{"add to cart", func(f *fakes) { f.cart.err = down }, func() *http.Request {
			return postForm("/cart", url.Values{"product_id": {"A"}, "quantity": {"1"}})
		}},
		{"empty cart", func(f *fakes) { f.cart.err = down }, func() *http.Request { return postForm("/cart/empty", nil) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, f := newTestFrontend()
			tt.setup(f)

			if w := serve(svc, tt.req()); w.Code != http.StatusServiceUnavailable {
				t.Errorf("status = %d, want 503", w.Code)
			}
		})
	}
}

func TestViewCart_invalidShippingCostIsAnError(t *testing.T) {
	svc, f := newTestFrontend()
	f.shipping.quote = &pb.Money{CurrencyCode: "USD", Units: 1, Nanos: -1}

	if w := serve(svc, httptest.NewRequest(http.MethodGet, "/cart", nil)); w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestPlaceOrder_failuresAfterPayingStillShowTheOrder(t *testing.T) {
	svc, f := newTestFrontend()
	f.checkout.order = &pb.OrderResult{OrderId: "order-1", ShippingCost: &pb.Money{CurrencyCode: "EUR", Units: 1}}
	// Not shown on the order page: an error page would invite retrying an already paid order.
	f.currency.listErr = status.Error(codes.Unavailable, "down")
	f.recommendations.err = status.Error(codes.Unavailable, "down")

	w := serve(svc, postForm("/cart/checkout", checkoutForm()))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	assertContains(t, w.Body.String(), "order-1")
}

func TestSingleSharedSession(t *testing.T) {
	t.Setenv("ENABLE_SINGLE_SHARED_SESSION", "true")
	var got string
	handler := ensureSessionID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = sessionID(r) }))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if got != "12345678-1234-1234-1234-123456789123" {
		t.Errorf("session ID = %q, want the one shared by every visitor", got)
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

func TestPlaceOrder_invalidTotal(t *testing.T) {
	svc, f := newTestFrontend()
	f.checkout.order = &pb.OrderResult{ShippingCost: &pb.Money{CurrencyCode: "EUR", Units: 1, Nanos: -1}}

	// An invalid amount must get an error page, not a panic.
	if w := serve(svc, postForm("/cart/checkout", checkoutForm())); w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestFooterShowsThePodButNotTheSessionID(t *testing.T) {
	t.Cleanup(func() { podHostname = "" })
	loadDeploymentDetails()
	if podHostname == "" {
		t.Fatal("the pod hostname shown in the footer is empty")
	}
	svc, _ := newTestFrontend()

	body := serve(svc, httptest.NewRequest(http.MethodGet, "/", nil)).Body.String()

	assertContains(t, body, podHostname)
	// HttpOnly would be pointless if the page showed the session ID.
	if strings.Contains(body, "user-1") {
		t.Error("the page shows the session ID")
	}
}

func TestSecurityHeaders(t *testing.T) {
	svc, _ := newTestFrontend()

	// Also on error pages and redirects, not only on rendered pages.
	for _, r := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/", nil),
		httptest.NewRequest(http.MethodGet, "/product/missing", nil),
		postForm("/cart/empty", nil),
	} {
		h := serve(svc, r).Header()
		if !strings.Contains(h.Get("Content-Security-Policy"), "script-src 'self'") ||
			h.Get("X-Content-Type-Options") != "nosniff" ||
			h.Get("X-Frame-Options") != "DENY" ||
			h.Get("Referrer-Policy") != "same-origin" {
			t.Errorf("%s %s: security headers missing: %v", r.Method, r.URL.Path, h)
		}
	}
}

func TestCrossOriginPostIsRejected(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		value    string
		wantCode int
	}{
		{"cross-site browser request", "Sec-Fetch-Site", "cross-site", http.StatusForbidden},
		{"other origin", "Origin", "http://evil.example", http.StatusForbidden},
		{"same-origin browser request", "Sec-Fetch-Site", "same-origin", http.StatusFound},
		{"no browser headers (curl, k6)", "", "", http.StatusFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, f := newTestFrontend()
			r := postForm("/cart", url.Values{"product_id": {"A"}, "quantity": {"1"}})
			if tt.header != "" {
				r.Header.Set(tt.header, tt.value)
			}

			if w := serve(svc, r); w.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", w.Code, tt.wantCode)
			}
			if (f.cart.added != nil) != (tt.wantCode == http.StatusFound) {
				t.Errorf("item added = %v, want it only for a same-origin request", f.cart.added != nil)
			}
		})
	}
}

func TestBodyLimit(t *testing.T) {
	svc, f := newTestFrontend()
	form := url.Values{"product_id": {"A"}, "quantity": {"1"}, "padding": {strings.Repeat("x", maxBodyBytes)}}

	// The form can't be read past the limit, so it fails validation.
	if w := serve(svc, postForm("/cart", form)); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
	if f.cart.added != nil {
		t.Error("an oversized request must not reach the cart")
	}
}

func TestNewServerTimeouts(t *testing.T) {
	srv := newServer(":0", http.NotFoundHandler(), 5*time.Second)

	if srv.ReadHeaderTimeout <= 0 || srv.ReadTimeout <= 0 || srv.IdleTimeout <= 0 {
		t.Errorf("server without read/idle timeouts: %+v", srv)
	}
	if srv.WriteTimeout <= 5*time.Second {
		t.Errorf("WriteTimeout = %v, want it longer than the request timeout, to still send the 504 page", srv.WriteTimeout)
	}
}

func TestLogsLeaveOutTheSessionID(t *testing.T) {
	svc, _ := newTestFrontend()
	var out bytes.Buffer
	logger := logrus.New()
	logger.Out = &out
	logger.SetLevel(logrus.DebugLevel)
	r := httptest.NewRequest(http.MethodGet, "/product/missing", nil)
	r.AddCookie(&http.Cookie{Name: cookieSessionID, Value: "session-1234"})

	newHandler(svc, logger, time.Minute).ServeHTTP(httptest.NewRecorder(), r)

	if out.Len() == 0 {
		t.Fatal("nothing was logged")
	}
	// The session cookie is the only key to the user's cart.
	if strings.Contains(out.String(), "session-1234") {
		t.Errorf("the session ID reached the logs:\n%s", out.String())
	}
}

func TestLogsLeaveOutHealthChecks(t *testing.T) {
	svc, _ := newTestFrontend()
	var out bytes.Buffer
	logger := logrus.New()
	logger.Out = &out
	logger.SetLevel(logrus.DebugLevel)
	w := httptest.NewRecorder()

	newHandler(svc, logger, time.Minute).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/_healthz", nil))

	if w.Code != http.StatusOK || w.Body.String() != "ok" {
		t.Errorf("health check = %d %q, want 200 \"ok\"", w.Code, w.Body.String())
	}
	if out.Len() != 0 {
		t.Errorf("the health check reached the logs:\n%s", out.String())
	}
}
