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
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/productcatalogservice/genproto"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

var (
	// catalogMutex guards productCatalog.catalog: requests read it while a
	// reload (see reloadCatalog) rewrites it.
	catalogMutex *sync.RWMutex
	log          *logrus.Logger
	extraLatency time.Duration

	port = "3550"

	// reloadCatalog is set and cleared by the signal handler while requests
	// read it.
	reloadCatalog atomic.Bool
)

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
	catalogMutex = &sync.RWMutex{}
}

func main() {
	// set injected latency
	if s := os.Getenv("EXTRA_LATENCY"); s != "" {
		v, err := time.ParseDuration(s)
		if err != nil {
			log.Fatalf("failed to parse EXTRA_LATENCY (%s) as time.Duration: %+v", s, err)
		}
		extraLatency = v
		log.Infof("extra latency enabled (duration: %v)", extraLatency)
	} else {
		extraLatency = time.Duration(0)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGUSR1, syscall.SIGUSR2)
	go func() {
		for {
			sig := <-sigs
			log.Printf("Received signal: %s", sig)
			if sig == syscall.SIGUSR1 {
				reloadCatalog.Store(true)
				log.Infof("Enable catalog reloading")
			} else {
				reloadCatalog.Store(false)
				log.Infof("Disable catalog reloading")
			}
		}
	}()

	if (os.Getenv("PORT") != "") == true {
		port = os.Getenv("PORT")
	}
	log.Infof("starting grpc server at :%s", port)
	if err := run(port); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func run(port string) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatal(err)
	}

	srv := grpc.NewServer()

	svc := &productCatalog{}
	err = loadCatalog(&svc.catalog)
	if err != nil {
		log.Fatalf("could not parse product catalog: %v", err)
	}

	pb.RegisterProductCatalogServiceServer(srv, svc)
	healthcheck := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthcheck)
	return serveUntilSignal(srv, listener, healthcheck)
}
