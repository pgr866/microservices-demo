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

package money

import (
	"errors"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/frontend/genproto"
)

const (
	nanosMin = -999999999
	nanosMax = +999999999
	nanosMod = 1000000000
)

var (
	ErrInvalidValue        = errors.New("one of the specified money values is invalid")
	ErrMismatchingCurrency = errors.New("mismatching currency codes")
)

// IsValid checks if specified value has a valid units/nanos signs and ranges.
func IsValid(m *pb.Money) bool {
	return signMatches(m) && validNanos(m.GetNanos())
}

func signMatches(m *pb.Money) bool {
	return m.GetNanos() == 0 || m.GetUnits() == 0 || (m.GetNanos() < 0) == (m.GetUnits() < 0)
}

func validNanos(nanos int32) bool { return nanosMin <= nanos && nanos <= nanosMax }

// Sum adds two values. Returns an error if one of the values are invalid or
// currency codes are not matching (unless currency code is unspecified for
// both).
func Sum(l, r *pb.Money) (*pb.Money, error) {
	if !IsValid(l) || !IsValid(r) {
		return nil, ErrInvalidValue
	} else if l.GetCurrencyCode() != r.GetCurrencyCode() {
		return nil, ErrMismatchingCurrency
	}
	units := l.GetUnits() + r.GetUnits()
	nanos := l.GetNanos() + r.GetNanos()

	if (units == 0 && nanos == 0) || (units > 0 && nanos >= 0) || (units < 0 && nanos <= 0) {
		// same sign <units, nanos>
		units += int64(nanos / nanosMod)
		nanos = nanos % nanosMod
	} else {
		// different sign. nanos guaranteed to not to go over the limit
		if units > 0 {
			units--
			nanos += nanosMod
		} else {
			units++
			nanos -= nanosMod
		}
	}

	return &pb.Money{
		Units:        units,
		Nanos:        nanos,
		CurrencyCode: l.GetCurrencyCode()}, nil
}

// MultiplySlow is a slow multiplication operation done through adding the value
// to itself n-1 times. Returns an error if the value is invalid.
func MultiplySlow(m *pb.Money, n uint32) (*pb.Money, error) {
	out := m
	for n > 1 {
		var err error
		out, err = Sum(out, m)
		if err != nil {
			return nil, err
		}
		n--
	}
	return out, nil
}
