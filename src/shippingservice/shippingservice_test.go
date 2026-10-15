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
	"regexp"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/shippingservice/genproto"
)

func TestGetQuote(t *testing.T) {
	tests := []struct {
		name      string
		items     []*pb.CartItem
		wantUnits int64
		wantNanos int32
	}{
		{"flat rate for any number of items", []*pb.CartItem{{ProductId: "23", Quantity: 1}, {ProductId: "46", Quantity: 3}}, 8, 990000000},
		{"empty cart ships free", nil, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := (&server{}).GetQuote(context.Background(), &pb.GetQuoteRequest{Items: tc.items})
			if err != nil {
				t.Fatal(err)
			}
			cost := res.GetCostUsd()
			if cost.GetCurrencyCode() != "USD" || cost.GetUnits() != tc.wantUnits || cost.GetNanos() != tc.wantNanos {
				t.Errorf("got %v, want USD %d.%09d", cost, tc.wantUnits, tc.wantNanos)
			}
		})
	}
}

func TestTrackingIdUniqueness(t *testing.T) {
	// 6.76e12 possible IDs: a repeat among 50 is under one in a billion.
	const n = 50
	seen := make(map[string]bool)
	for range n {
		seen[CreateTrackingId("same-salt")] = true
	}
	if len(seen) != n {
		t.Errorf("CreateTrackingId: got %d distinct values out of %d, want all unique", len(seen), n)
	}
}

func TestCreateQuoteFromFloat(t *testing.T) {
	tests := []struct {
		name    string
		value   float64
		dollars uint32
		cents   uint32
	}{
		{"zero", 0.0, 0, 0},
		{"whole dollars", 10.0, 10, 0},
		{"with cents", 8.99, 8, 99},
		{"small value", 0.50, 0, 50},
		{"large value", 100.01, 100, 1},
		{"cents not exact in floating point", 0.29, 0, 29},
		{"rounding carries into dollars", 8.999, 9, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := CreateQuoteFromFloat(tc.value)
			if q.Dollars != tc.dollars || q.Cents != tc.cents {
				t.Errorf("CreateQuoteFromFloat(%v) = $%d.%d, want $%d.%d",
					tc.value, q.Dollars, q.Cents, tc.dollars, tc.cents)
			}
		})
	}
}

func TestGetRandomLetterCode(t *testing.T) {
	for range 100 {
		code := getRandomLetterCode()
		if code < 65 || code > 90 {
			t.Errorf("getRandomLetterCode: got %d (%c), expected range 65-90 (A-Z)", code, code)
		}
	}
}

func TestGetRandomNumber(t *testing.T) {
	for _, digits := range []int{1, 3, 5, 7, 10} {
		result := getRandomNumber(digits)
		if len(result) != digits {
			t.Errorf("getRandomNumber(%d) = '%s' (len %d), expected length %d",
				digits, result, len(result), digits)
		}
		for _, c := range result {
			if c < '0' || c > '9' {
				t.Errorf("getRandomNumber(%d) = %q, want only digits", digits, result)
				break
			}
		}
	}
}

func TestGetQuoteInvalidQuantity(t *testing.T) {
	for _, quantity := range []int32{0, -1} {
		// The negative item would cancel out the other one and make the shipping free.
		req := &pb.GetQuoteRequest{Items: []*pb.CartItem{
			{ProductId: "A", Quantity: 1},
			{ProductId: "B", Quantity: quantity},
		}}

		_, err := (&server{}).GetQuote(context.Background(), req)
		if got := status.Code(err); got != codes.InvalidArgument {
			t.Errorf("quantity %d: status code = %v, want %v", quantity, got, codes.InvalidArgument)
		}
	}
}

func TestShipOrderWithoutAddress(t *testing.T) {
	s := server{}

	// A nil address must not panic, which would take down the whole server.
	_, err := s.ShipOrder(context.Background(), &pb.ShipOrderRequest{})
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Errorf("status code = %v, want %v", got, codes.InvalidArgument)
	}
}

func TestShipOrderTrackingIdEncodesAddress(t *testing.T) {
	s := server{}

	// "Muffin Man, London, " is 20 characters, so the numeric groups carry 20 and 10.
	req := &pb.ShipOrderRequest{Address: &pb.Address{StreetAddress: "Muffin Man", City: "London"}}
	pattern := regexp.MustCompile(`^[A-Z]{2}-20\d{3}-10\d{7}$`)

	res, err := s.ShipOrder(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !pattern.MatchString(res.GetTrackingId()) {
		t.Errorf("tracking ID '%s' does not match '%s'", res.GetTrackingId(), pattern)
	}
}
