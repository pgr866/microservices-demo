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

// shutdownTimeout is how long in-flight calls get to finish on SIGTERM: less
// than the 30 s Kubernetes waits by default before sending SIGKILL.
const shutdownTimeout = 10 * time.Second

// serveUntilSignal serves until SIGTERM (what Kubernetes sends to delete a pod)
// or SIGINT, then reports NOT_SERVING to the probes, lets in-flight calls
// finish and returns nil. Without it the process died on the spot, cutting
// them.
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

// stopper is the part of *grpc.Server that gracefulStop uses.
type stopper interface {
	GracefulStop()
	Stop()
}

// gracefulStop lets in-flight calls finish, but no longer than timeout: then it
// cuts the rest, so the process still exits before Kubernetes kills it.
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
