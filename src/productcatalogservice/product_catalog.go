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
	"time"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/productcatalogservice/genproto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type productCatalog struct {
	pb.UnimplementedProductCatalogServiceServer
	catalog pb.ListProductsResponse
}

func (p *productCatalog) ListProducts(context.Context, *pb.Empty) (*pb.ListProductsResponse, error) {
	time.Sleep(extraLatency)

	return &pb.ListProductsResponse{Products: p.parseCatalog()}, nil
}

func (p *productCatalog) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.Product, error) {
	time.Sleep(extraLatency)

	catalog := p.parseCatalog()
	for _, product := range catalog {
		if req.Id == product.Id {
			return product, nil
		}
	}

	return nil, status.Errorf(codes.NotFound, "no product with ID %s", req.Id)
}

func (p *productCatalog) SearchProducts(ctx context.Context, req *pb.SearchProductsRequest) (*pb.SearchProductsResponse, error) {
	time.Sleep(extraLatency)

	var ps []*pb.Product
	for _, product := range p.parseCatalog() {
		if strings.Contains(strings.ToLower(product.Name), strings.ToLower(req.Query)) ||
			strings.Contains(strings.ToLower(product.Description), strings.ToLower(req.Query)) {
			ps = append(ps, product)
		}
	}

	return &pb.SearchProductsResponse{Results: ps}, nil
}

func (p *productCatalog) parseCatalog() []*pb.Product {
	catalogMutex.RLock()
	empty := len(p.catalog.Products) == 0
	catalogMutex.RUnlock()

	if reloadCatalog.Load() || empty {
		err := loadCatalog(&p.catalog)
		if err != nil {
			return []*pb.Product{}
		}
	}

	catalogMutex.RLock()
	defer catalogMutex.RUnlock()
	return p.catalog.Products
}
