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

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/checkoutservice/genproto"
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

// IsValid reports whether units and nanos have matching signs and nanos is in range.
func IsValid(m *pb.Money) bool {
	return signMatches(m) && validNanos(m.GetNanos())
}

func signMatches(m *pb.Money) bool {
	return m.GetNanos() == 0 || m.GetUnits() == 0 || (m.GetNanos() < 0) == (m.GetUnits() < 0)
}

func validNanos(nanos int32) bool { return nanosMin <= nanos && nanos <= nanosMax }

// Sum adds two values of the same currency.
func Sum(l, r *pb.Money) (*pb.Money, error) {
	if !IsValid(l) || !IsValid(r) {
		return nil, ErrInvalidValue
	} else if l.GetCurrencyCode() != r.GetCurrencyCode() {
		return nil, ErrMismatchingCurrency
	}
	units := l.GetUnits() + r.GetUnits()
	nanos := l.GetNanos() + r.GetNanos()

	// With no whole units the nanos set the sign; borrowing would turn 0.3+0.3 into 1-0.4.
	if units == 0 || nanos == 0 || (units > 0) == (nanos > 0) {
		units += int64(nanos / nanosMod)
		nanos = nanos % nanosMod
	} else {
		// Different signs: the inputs are valid, so |nanos| < nanosMod.
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

// MultiplySlow multiplies by repeated addition.
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
