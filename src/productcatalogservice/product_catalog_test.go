// Copyright 2023 Google LLC
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
	"slices"
	"testing"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/productcatalogservice/genproto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newTestCatalog() *productCatalog {
	return &productCatalog{catalog: pb.ListProductsResponse{Products: []*pb.Product{
		{Id: "1", Name: "Vintage Typewriter", Description: "Old school."},
		{Id: "2", Name: "Mug", Description: "A simple TYPEWRITER-shaped mug."},
		{Id: "3", Name: "Candle", Description: "Smells nice."},
	}}}
}

func productIDs(products []*pb.Product) []string {
	var ids []string
	for _, p := range products {
		ids = append(ids, p.GetId())
	}
	return ids
}

func TestListProducts(t *testing.T) {
	res, err := newTestCatalog().ListProducts(context.Background(), &pb.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := productIDs(res.GetProducts()), []string{"1", "2", "3"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGetProduct(t *testing.T) {
	p := newTestCatalog()

	product, err := p.GetProduct(context.Background(), &pb.GetProductRequest{Id: "2"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := product.GetName(), "Mug"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	for _, id := range []string{"4", ""} {
		_, err := p.GetProduct(context.Background(), &pb.GetProductRequest{Id: id})
		if got, want := status.Code(err), codes.NotFound; got != want {
			t.Errorf("ID %q: got %v, want %v", id, got, want)
		}
	}
}

func TestSearchProducts(t *testing.T) {
	p := newTestCatalog()

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"matches name and description ignoring case", "typewriter", []string{"1", "2"}},
		{"matches description only", "smells", []string{"3"}},
		{"upper-case query", "CANDLE", []string{"3"}},
		{"empty query matches everything", "", []string{"1", "2", "3"}},
		{"no match", "bicycle", nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := p.SearchProducts(context.Background(), &pb.SearchProductsRequest{Query: tc.query})
			if err != nil {
				t.Fatal(err)
			}
			if got := productIDs(res.GetResults()); !slices.Equal(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
