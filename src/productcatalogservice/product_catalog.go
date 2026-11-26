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
	"strings"
	"sync"
	"time"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/productcatalogservice/genproto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type productCatalog struct {
	pb.UnimplementedProductCatalogServiceServer

	// mu guards catalog against concurrent reloads.
	mu      sync.RWMutex
	catalog pb.ListProductsResponse
}

func (p *productCatalog) ListProducts(context.Context, *pb.Empty) (*pb.ListProductsResponse, error) {
	time.Sleep(extraLatency)

	return &pb.ListProductsResponse{Products: p.parseCatalog()}, nil
}

func (p *productCatalog) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.Product, error) {
	time.Sleep(extraLatency)

	for _, product := range p.parseCatalog() {
		if product.GetId() == req.GetId() {
			return product, nil
		}
	}

	return nil, status.Errorf(codes.NotFound, "no product with ID %q", req.GetId())
}

func (p *productCatalog) SearchProducts(ctx context.Context, req *pb.SearchProductsRequest) (*pb.SearchProductsResponse, error) {
	time.Sleep(extraLatency)

	query := strings.ToLower(req.GetQuery())
	var ps []*pb.Product
	for _, product := range p.parseCatalog() {
		if strings.Contains(strings.ToLower(product.GetName()), query) ||
			strings.Contains(strings.ToLower(product.GetDescription()), query) {
			ps = append(ps, product)
		}
	}

	return &pb.SearchProductsResponse{Results: ps}, nil
}

// parseCatalog reloads products.json if empty or reloading is on; a failed reload keeps the old catalog.
func (p *productCatalog) parseCatalog() []*pb.Product {
	p.mu.RLock()
	empty := len(p.catalog.Products) == 0
	p.mu.RUnlock()

	if reloadCatalog.Load() || empty {
		// Parsed outside the lock so requests don't block on the file.
		var fresh pb.ListProductsResponse
		if err := loadCatalog(&fresh); err == nil {
			p.mu.Lock()
			p.catalog.Products = fresh.Products
			p.mu.Unlock()
		}
	}

	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.catalog.Products
}
