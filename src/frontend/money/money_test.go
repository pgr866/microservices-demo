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
	"testing"

	"google.golang.org/protobuf/proto"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/frontend/genproto"
)

func mmc(u int64, n int32, c string) *pb.Money { return &pb.Money{Units: u, Nanos: n, CurrencyCode: c} }
func mm(u int64, n int32) *pb.Money            { return mmc(u, n, "") }

func TestIsValid(t *testing.T) {
	tests := []struct {
		name string
		in   *pb.Money
		want bool
	}{
		{"valid -/-", mm(-981273891273, -999999999), true},
		{"invalid -/+", mm(-981273891273, +999999999), false},
		{"valid +/+", mm(981273891273, 999999999), true},
		{"invalid +/-", mm(981273891273, -999999999), false},
		{"invalid +/+overflow", mm(3, 1000000000), false},
		{"invalid +/-overflow", mm(3, -1000000000), false},
		{"invalid -/+overflow", mm(-3, 1000000000), false},
		{"invalid -/-overflow", mm(-3, -1000000000), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValid(tt.in); got != tt.want {
				t.Errorf("IsValid(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestSum(t *testing.T) {
	type args struct {
		l *pb.Money
		r *pb.Money
	}
	tests := []struct {
		name    string
		args    args
		want    *pb.Money
		wantErr error
	}{
		{"0+0=0", args{mm(0, 0), mm(0, 0)}, mm(0, 0), nil},
		{"Error: currency code on left", args{mmc(0, 0, "XXX"), mm(0, 0)}, nil, ErrMismatchingCurrency},
		{"Error: currency code on right", args{mm(0, 0), mmc(0, 0, "YYY")}, nil, ErrMismatchingCurrency},
		{"Error: currency code mismatch", args{mmc(0, 0, "AAA"), mmc(0, 0, "BBB")}, nil, ErrMismatchingCurrency},
		{"Error: invalid +/-", args{mm(+1, -1), mm(0, 0)}, nil, ErrInvalidValue},
		{"Error: invalid -/+", args{mm(0, 0), mm(-1, +2)}, nil, ErrInvalidValue},
		{"Error: invalid nanos", args{mm(0, 1000000000), mm(1, 0)}, nil, ErrInvalidValue},
		{"both positive (no carry)", args{mm(2, 200000000), mm(2, 200000000)}, mm(4, 400000000), nil},
		{"both positive (nanos=max)", args{mm(2, 111111111), mm(2, 888888888)}, mm(4, 999999999), nil},
		{"both positive (carry)", args{mm(2, 200000000), mm(2, 900000000)}, mm(5, 100000000), nil},
		{"both negative (no carry)", args{mm(-2, -200000000), mm(-2, -200000000)}, mm(-4, -400000000), nil},
		{"both negative (carry)", args{mm(-2, -200000000), mm(-2, -900000000)}, mm(-5, -100000000), nil},
		{"mixed (larger positive, just decimals)", args{mm(11, 0), mm(-2, 0)}, mm(9, 0), nil},
		{"mixed (larger negative, just decimals)", args{mm(-11, 0), mm(2, 0)}, mm(-9, 0), nil},
		{"mixed (larger positive, no borrow)", args{mm(11, 100000000), mm(-2, -100000000)}, mm(9, 0), nil},
		{"mixed (larger positive, with borrow)", args{mm(11, 100000000), mm(-2, -9000000 /*.09*/)}, mm(9, 91000000 /*.091*/), nil},
		{"mixed (larger negative, no borrow)", args{mm(-11, -100000000), mm(2, 100000000)}, mm(-9, 0), nil},
		{"mixed (larger negative, with borrow)", args{mm(-11, -100000000), mm(2, 9000000 /*.09*/)}, mm(-9, -91000000 /*.091*/), nil},
		{"mixed (positive units, negative nanos)", args{mm(1, 100000000), mm(0, -500000000)}, mm(0, 600000000), nil},
		{"mixed (negative units, positive nanos)", args{mm(-1, -100000000), mm(0, 500000000)}, mm(0, -600000000), nil},
		{"0+negative", args{mm(0, 0), mm(-2, -100000000)}, mm(-2, -100000000), nil},
		{"negative+0", args{mm(-2, -100000000), mm(0, 0)}, mm(-2, -100000000), nil},
		{"keeps currency code", args{mmc(1, 0, "EUR"), mmc(2, 0, "EUR")}, mmc(3, 0, "EUR"), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Sum(tt.args.l, tt.args.r)
			if err != tt.wantErr {
				t.Errorf("Sum([%v],[%v]): expected err=\"%v\" got=\"%v\"", tt.args.l, tt.args.r, tt.wantErr, err)
			}
			if !proto.Equal(got, tt.want) {
				t.Errorf("Sum([%v],[%v]) = %v, want %v", tt.args.l, tt.args.r, got, tt.want)
			}
		})
	}
}

func TestMultiplySlow(t *testing.T) {
	tests := []struct {
		name    string
		in      *pb.Money
		n       uint32
		want    *pb.Money
		wantErr error
	}{
		{"times 1", mmc(3, 500000000, "USD"), 1, mmc(3, 500000000, "USD"), nil},
		{"times 3 (carry)", mmc(3, 500000000, "USD"), 3, mmc(10, 500000000, "USD"), nil},
		{"zero", mmc(0, 0, "USD"), 5, mmc(0, 0, "USD"), nil},
		{"Error: invalid value", mmc(1, -1, "USD"), 2, nil, ErrInvalidValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MultiplySlow(tt.in, tt.n)
			if err != tt.wantErr {
				t.Errorf("MultiplySlow(%v, %d): expected err=\"%v\" got=\"%v\"", tt.in, tt.n, tt.wantErr, err)
			}
			if !proto.Equal(got, tt.want) {
				t.Errorf("MultiplySlow(%v, %d) = %v, want %v", tt.in, tt.n, got, tt.want)
			}
		})
	}
}

func TestMultiplySlow_doesNotModifyInput(t *testing.T) {
	in := mmc(2, 0, "USD")
	if _, err := MultiplySlow(in, 4); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(in, mmc(2, 0, "USD")) {
		t.Errorf("MultiplySlow modified its input: %v", in)
	}
}
