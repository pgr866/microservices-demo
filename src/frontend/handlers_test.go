// Copyright 2024 Google LLC
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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/frontend/genproto"
)

func TestSetPlatformDetails(t *testing.T) {
	tests := []struct {
		name         string
		env          string
		wantProvider string
		wantCSS      string
	}{
		{"aws", "aws", "AWS", "aws-platform"},
		{"onprem", "onprem", "On-Premises", "onprem-platform"},
		{"azure", "azure", "Azure", "azure-platform"},
		{"gcp", "gcp", "Google Cloud", "gcp-platform"},
		{"alibaba", "alibaba", "Alibaba Cloud", "alibaba-platform"},
		{"local", "local", "local", "local"},
		{"unknown falls back to local", "made-up-env", "local", "local"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var plat platformDetails
			plat.setPlatformDetails(tt.env)
			if plat.provider != tt.wantProvider || plat.css != tt.wantCSS {
				t.Errorf("setPlatformDetails(%q) = {%q, %q}, want {%q, %q}",
					tt.env, plat.provider, plat.css, tt.wantProvider, tt.wantCSS)
			}
		})
	}
}

func TestCartSize(t *testing.T) {
	tests := []struct {
		name string
		in   []*pb.CartItem
		want int
	}{
		{"empty cart", nil, 0},
		{"single item", []*pb.CartItem{{ProductId: "A", Quantity: 3}}, 3},
		{"multiple items", []*pb.CartItem{{ProductId: "A", Quantity: 2}, {ProductId: "B", Quantity: 5}}, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cartSize(tt.in); got != tt.want {
				t.Errorf("cartSize(%v) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestCartIDs(t *testing.T) {
	tests := []struct {
		name string
		in   []*pb.CartItem
		want []string
	}{
		{"empty cart", nil, []string{}},
		{"multiple items", []*pb.CartItem{{ProductId: "A"}, {ProductId: "B"}}, []string{"A", "B"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cartIDs(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("cartIDs(%v) = %v, want %v", tt.in, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("cartIDs(%v)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestRenderCurrencyLogo(t *testing.T) {
	tests := []struct {
		currency string
		want     string
	}{
		{"USD", "$"},
		{"EUR", "€"},
		{"JPY", "¥"},
		{"unknown currency defaults to $", "$"},
	}
	for _, tt := range tests {
		t.Run(tt.currency, func(t *testing.T) {
			if got := renderCurrencyLogo(tt.currency); got != tt.want {
				t.Errorf("renderCurrencyLogo(%q) = %q, want %q", tt.currency, got, tt.want)
			}
		})
	}
}

func TestRenderMoney(t *testing.T) {
	tests := []struct {
		name string
		in   *pb.Money
		want string
	}{
		{"whole units", &pb.Money{CurrencyCode: "USD", Units: 5, Nanos: 0}, "$5.00"},
		{"with cents", &pb.Money{CurrencyCode: "EUR", Units: 12, Nanos: 340000000}, "€12.34"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderMoney(tt.in); got != tt.want {
				t.Errorf("renderMoney(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestHTTPStatusFromCode(t *testing.T) {
	tests := []struct {
		code codes.Code
		want int
	}{
		{codes.OK, http.StatusOK},
		{codes.Canceled, 499},
		{codes.Unknown, http.StatusInternalServerError},
		{codes.InvalidArgument, http.StatusBadRequest},
		{codes.DeadlineExceeded, http.StatusGatewayTimeout},
		{codes.NotFound, http.StatusNotFound},
		{codes.AlreadyExists, http.StatusConflict},
		{codes.PermissionDenied, http.StatusForbidden},
		{codes.ResourceExhausted, http.StatusTooManyRequests},
		{codes.FailedPrecondition, http.StatusBadRequest},
		{codes.Aborted, http.StatusConflict},
		{codes.OutOfRange, http.StatusBadRequest},
		{codes.Unimplemented, http.StatusNotImplemented},
		{codes.Internal, http.StatusInternalServerError},
		{codes.Unavailable, http.StatusServiceUnavailable},
		{codes.DataLoss, http.StatusInternalServerError},
		{codes.Unauthenticated, http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.code.String(), func(t *testing.T) {
			if got := httpStatusFromCode(tt.code); got != tt.want {
				t.Errorf("httpStatusFromCode(%v) = %d, want %d", tt.code, got, tt.want)
			}
		})
	}
}

func renderError(t *testing.T, err error, code int) *httptest.ResponseRecorder {
	t.Helper()
	logger := logrus.New()
	logger.Out = io.Discard
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(context.WithValue(r.Context(), ctxKeyRequestID{}, "req-42"))
	w := httptest.NewRecorder()
	renderHTTPError(logger, r, w, err, code)
	return w
}

func TestRenderHTTPError_clientErrorShowsTheServiceReason(t *testing.T) {
	err := fmt.Errorf("failed to complete the order: %w", status.Error(codes.InvalidArgument, "Your credit card expired"))

	w := renderError(t, err, http.StatusInternalServerError)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d from the gRPC code", w.Code, http.StatusBadRequest)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Your credit card expired") {
		t.Error("page does not show the reason the order was rejected")
	}
	if strings.Contains(body, "rpc error") || strings.Contains(body, "failed to complete the order") {
		t.Error("page shows the internal error chain instead of just the reason")
	}
}

func TestRenderHTTPError_serverErrorHidesDetails(t *testing.T) {
	err := fmt.Errorf("failed to complete the order: %w", status.Error(codes.Unavailable, "dial tcp 172.18.0.2:50052: i/o timeout"))

	w := renderError(t, err, http.StatusInternalServerError)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d from the gRPC code", w.Code, http.StatusServiceUnavailable)
	}
	body := w.Body.String()
	for _, leak := range []string{"172.18.0.2", "rpc error", "failed to complete the order", ".go:"} {
		if strings.Contains(body, leak) {
			t.Errorf("page leaks internal detail %q", leak)
		}
	}
	if !strings.Contains(body, "req-42") {
		t.Error("page does not show the request ID to find the error in the log")
	}
}

func TestRenderHTTPError_nonGRPCErrorKeepsHandlerStatus(t *testing.T) {
	w := renderError(t, errors.New("product id not specified"), http.StatusBadRequest)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want the handler's %d", w.Code, http.StatusBadRequest)
	}
	if !strings.Contains(w.Body.String(), "product id not specified") {
		t.Error("page does not show the reason of the client error")
	}
}

func TestWithTimeout(t *testing.T) {
	var deadline time.Time
	var ok bool
	handler := withTimeout(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		deadline, ok = r.Context().Deadline()
	}), 2*time.Second)

	before := time.Now()
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	after := time.Now()

	if !ok {
		t.Fatal("request context has no deadline")
	}
	if deadline.Before(before.Add(2*time.Second)) || deadline.After(after.Add(2*time.Second)) {
		t.Errorf("deadline %v, want 2s after the request started", deadline)
	}
}
