// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"testing"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/productcatalogservice/genproto"
)

func TestLoadCatalogFromLocalFile_ParsesRealProductsFile(t *testing.T) {
	var catalog pb.ListProductsResponse

	if err := loadCatalogFromLocalFile(&catalog); err != nil {
		t.Fatalf("expected no error loading products.json, got: %v", err)
	}

	if len(catalog.Products) == 0 {
		t.Fatal("expected products.json to parse into at least one product")
	}
}

func TestLoadCatalogFromLocalFile_MissingFile(t *testing.T) {
	t.Chdir(t.TempDir())

	var catalog pb.ListProductsResponse

	if err := loadCatalogFromLocalFile(&catalog); err == nil {
		t.Fatal("expected an error when products.json does not exist in the working directory")
	}
}

func TestLoadCatalog_LocksAndParses(t *testing.T) {
	var catalog pb.ListProductsResponse

	if err := loadCatalog(&catalog); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(catalog.Products) == 0 {
		t.Fatal("expected products.json to parse into at least one product")
	}
}

func TestParseCatalog_ReloadsWhenEmpty(t *testing.T) {
	p := &productCatalog{}

	products := p.parseCatalog()

	if len(products) == 0 {
		t.Fatal("expected parseCatalog to load products.json when the catalog is empty")
	}
}

func TestParseCatalog_ReloadsWhenFlagSet(t *testing.T) {
	p := &productCatalog{
		catalog: pb.ListProductsResponse{
			Products: []*pb.Product{{Id: "stale", Name: "Stale Product"}},
		},
	}

	reloadCatalog = true
	t.Cleanup(func() { reloadCatalog = false })

	products := p.parseCatalog()

	if len(products) == 0 {
		t.Fatal("expected parseCatalog to reload from products.json when reloadCatalog is set")
	}
	for _, product := range products {
		if product.Id == "stale" {
			t.Fatal("expected the stale in-memory catalog to be replaced by the reload")
		}
	}
}

func TestParseCatalog_ReturnsEmptyOnLoadError(t *testing.T) {
	t.Chdir(t.TempDir())

	p := &productCatalog{}

	products := p.parseCatalog()

	if len(products) != 0 {
		t.Fatalf("expected an empty catalog when products.json cannot be loaded, got %d products", len(products))
	}
}
