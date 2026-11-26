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
	"cmp"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
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
	log          *logrus.Logger
	extraLatency time.Duration

	port = "3550"

	// reloadCatalog is toggled by the signal handler while requests read it.
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
	log.AddHook(loggerName("productcatalogservice"))
	level, err := parseLogLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		log.Fatal(err)
	}
	log.Level = level
}

// loggerName adds the "logger" field to every log line.
type loggerName string

func (loggerName) Levels() []logrus.Level { return logrus.AllLevels }

func (n loggerName) Fire(entry *logrus.Entry) error {
	entry.Data["logger"] = string(n)
	return nil
}

// parseLogLevel accepts only the levels shared by every service (logrus.ParseLevel also takes trace, fatal, panic).
func parseLogLevel(s string) (logrus.Level, error) {
	switch strings.ToLower(cmp.Or(s, "info")) {
	case "debug":
		return logrus.DebugLevel, nil
	case "info":
		return logrus.InfoLevel, nil
	case "warn":
		return logrus.WarnLevel, nil
	case "error":
		return logrus.ErrorLevel, nil
	}
	return 0, fmt.Errorf("invalid LOG_LEVEL %q: want debug, info, warn or error", s)
}

func main() {
	if s := os.Getenv("EXTRA_LATENCY"); s != "" {
		v, err := time.ParseDuration(s)
		if err != nil {
			log.Fatalf("failed to parse EXTRA_LATENCY (%s) as time.Duration: %+v", s, err)
		}
		extraLatency = v
		log.Infof("extra latency enabled (duration: %v)", extraLatency)
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
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}

	srv := grpc.NewServer()

	svc := &productCatalog{}
	err = loadCatalog(&svc.catalog)
	if err != nil {
		log.Fatalf("could not parse product catalog: %v", err)
	}
	log.Infof("catalog loaded (%d products)", len(svc.catalog.GetProducts()))

	pb.RegisterProductCatalogServiceServer(srv, svc)
	healthcheck := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthcheck)
	return serveUntilSignal(srv, listener, healthcheck)
}
