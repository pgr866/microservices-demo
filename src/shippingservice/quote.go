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

import "math"

type Quote struct {
	Dollars uint32
	Cents   uint32
}

func CreateQuoteFromCount(count int) Quote {
	if count == 0 {
		return CreateQuoteFromFloat(0)
	}
	return CreateQuoteFromFloat(8.99)
}

func CreateQuoteFromFloat(value float64) Quote {
	// Round, not truncate: 0.29 * 100 is 28.999999999999996 in floating point.
	cents := uint32(math.Round(value * 100))
	return Quote{cents / 100, cents % 100}
}
