package main

import (
	"testing"
	"time"
)

// fakeServer's GracefulStop returns once release is closed, as the real one
// does once the calls in flight finish (or Stop cuts them).
type fakeServer struct {
	release chan struct{}
	stopped bool
}

func (f *fakeServer) GracefulStop() { <-f.release }

func (f *fakeServer) Stop() {
	f.stopped = true
	close(f.release)
}

func TestGracefulStop_callsFinishInTime(t *testing.T) {
	srv := &fakeServer{release: make(chan struct{})}
	close(srv.release)

	gracefulStop(srv, time.Second)

	if srv.stopped {
		t.Error("Stop was called although the calls in flight finished in time")
	}
}

func TestGracefulStop_cutsCallsAfterTimeout(t *testing.T) {
	srv := &fakeServer{release: make(chan struct{})}

	start := time.Now()
	gracefulStop(srv, 10*time.Millisecond)

	if !srv.stopped {
		t.Error("Stop was not called after the timeout")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("gracefulStop took %v, want about the 10ms timeout", elapsed)
	}
}
