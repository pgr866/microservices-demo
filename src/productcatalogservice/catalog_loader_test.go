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
	"os"
	"slices"
	"testing"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/productcatalogservice/genproto"
)

func TestLoadCatalog_parsesProductsFile(t *testing.T) {
	var catalog pb.ListProductsResponse

	if err := loadCatalog(&catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.GetProducts()) == 0 {
		t.Fatal("products.json parsed into no products")
	}
}

func TestLoadCatalog_errors(t *testing.T) {
	tests := []struct {
		name     string
		contents *string // nil: no products.json at all
	}{
		{"missing file", nil},
		{"malformed JSON", new(`{"products": [`)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if tc.contents != nil {
				if err := os.WriteFile("products.json", []byte(*tc.contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			var catalog pb.ListProductsResponse
			if err := loadCatalog(&catalog); err == nil {
				t.Fatal("got no error")
			}
		})
	}
}

func TestParseCatalog_loadsWhenEmpty(t *testing.T) {
	p := &productCatalog{}

	if len(p.parseCatalog()) == 0 {
		t.Fatal("parseCatalog did not load products.json into the empty catalog")
	}
}

func TestParseCatalog_reload(t *testing.T) {
	reloadCatalog.Store(true)
	t.Cleanup(func() { reloadCatalog.Store(false) })

	p := newTestCatalog()
	if got := p.parseCatalog(); slices.Contains(productIDs(got), "1") {
		t.Errorf("parseCatalog kept the old catalog instead of reloading products.json: %v", productIDs(got))
	}

	// A reload that fails keeps the catalog it had, instead of emptying it.
	t.Chdir(t.TempDir())
	p = newTestCatalog()
	if got, want := productIDs(p.parseCatalog()), []string{"1", "2", "3"}; !slices.Equal(got, want) {
		t.Errorf("after a failed reload got %v, want %v", got, want)
	}
}

func TestParseCatalog_emptyWhenFirstLoadFails(t *testing.T) {
	t.Chdir(t.TempDir())
	p := &productCatalog{}

	if got := p.parseCatalog(); len(got) != 0 {
		t.Fatalf("got %d products, want none", len(got))
	}
}
