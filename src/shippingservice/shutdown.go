package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
)

// shutdownTimeout stays below the 30 s Kubernetes waits before SIGKILL.
const shutdownTimeout = 10 * time.Second

// serveUntilSignal serves until SIGTERM/SIGINT, then reports NOT_SERVING and drains in-flight calls.
func serveUntilSignal(srv *grpc.Server, lis net.Listener, healthcheck *health.Server) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	stopped := make(chan struct{})
	go func() {
		<-ctx.Done()
		log.Info("shutting down")
		healthcheck.Shutdown()
		gracefulStop(srv, shutdownTimeout)
		close(stopped)
	}()

	if err := srv.Serve(lis); err != nil {
		return err
	}
	// Serve returns as soon as the listener closes: wait for the calls in flight.
	<-stopped
	return nil
}

// stopper lets the tests replace *grpc.Server.
type stopper interface {
	GracefulStop()
	Stop()
}

// gracefulStop drains in-flight calls for up to timeout, then cuts the rest.
func gracefulStop(srv stopper, timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		log.Warnf("calls still in flight after %v, cutting them", timeout)
		srv.Stop()
		<-done
	}
}
