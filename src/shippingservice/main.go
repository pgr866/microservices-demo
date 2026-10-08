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

import (
	"cmp"
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/status"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/shippingservice/genproto"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const (
	defaultPort = "50051"
)

var log *logrus.Logger

func init() {
	log = logrus.New()
	log.Formatter = &logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "severity",
			logrus.FieldKeyMsg:   "message",
		},
		TimestampFormat: time.RFC3339Nano,
	}
	log.Out = os.Stdout
	level, err := logrus.ParseLevel(cmp.Or(os.Getenv("LOG_LEVEL"), "info"))
	if err != nil {
		log.Fatalf("invalid LOG_LEVEL: %v", err)
	}
	log.Level = level
}

func main() {
	port := defaultPort
	if value, ok := os.LookupEnv("PORT"); ok == true {
		port = value
	}
	port = fmt.Sprintf(":%s", port)

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	srv := grpc.NewServer()
	svc := &server{}
	pb.RegisterShippingServiceServer(srv, svc)
	healthcheck := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthcheck)
	log.Infof("Shipping Service listening on port %s", port)

	if err := serveUntilSignal(srv, lis, healthcheck); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

type server struct {
	pb.UnimplementedShippingServiceServer
}

func (s *server) GetQuote(ctx context.Context, in *pb.GetQuoteRequest) (*pb.GetQuoteResponse, error) {
	log.Debug("[GetQuote] received request")
	defer log.Debug("[GetQuote] completed request")

	count := 0
	for _, item := range in.GetItems() {
		// Otherwise a negative quantity could cancel out the rest and make
		// the shipping free.
		if item.GetQuantity() < 1 {
			return nil, status.Errorf(codes.InvalidArgument, "invalid quantity %d for product %q", item.GetQuantity(), item.GetProductId())
		}
		count += int(item.GetQuantity())
	}
	quote := CreateQuoteFromCount(count)

	return &pb.GetQuoteResponse{
		CostUsd: &pb.Money{
			CurrencyCode: "USD",
			Units:        int64(quote.Dollars),
			Nanos:        int32(quote.Cents * 10000000)},
	}, nil

}

// ShipOrder ships nothing: it only returns a made-up tracking ID.
func (s *server) ShipOrder(ctx context.Context, in *pb.ShipOrderRequest) (*pb.ShipOrderResponse, error) {
	log.Debug("[ShipOrder] received request")
	defer log.Debug("[ShipOrder] completed request")
	// Without this check, a request with no address dereferences a nil pointer,
	// and the panic takes down the whole server, not just this call.
	if in.GetAddress() == nil {
		return nil, status.Error(codes.InvalidArgument, "address is required")
	}
	baseAddress := fmt.Sprintf("%s, %s, %s", in.GetAddress().GetStreetAddress(), in.GetAddress().GetCity(), in.GetAddress().GetState())
	id := CreateTrackingId(baseAddress)

	return &pb.ShipOrderResponse{
		TrackingId: id,
	}, nil
}
