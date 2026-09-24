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
	"testing"

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

func TestStringinSlice(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		val   string
		want  bool
	}{
		{"present", validEnvs, "azure", true},
		{"absent", validEnvs, "not-a-real-env", false},
		{"empty slice", []string{}, "azure", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stringinSlice(tt.slice, tt.val); got != tt.want {
				t.Errorf("stringinSlice(%v, %q) = %v, want %v", tt.slice, tt.val, got, tt.want)
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
		in   pb.Money
		want string
	}{
		{"whole units", pb.Money{CurrencyCode: "USD", Units: 5, Nanos: 0}, "$5.00"},
		{"with cents", pb.Money{CurrencyCode: "EUR", Units: 12, Nanos: 340000000}, "€12.34"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderMoney(tt.in); got != tt.want {
				t.Errorf("renderMoney(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
