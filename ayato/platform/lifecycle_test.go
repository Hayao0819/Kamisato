package platform

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestReadinessStartsUnready(t *testing.T) {
	t.Parallel()
	if (&Readiness{}).Ready() {
		t.Fatal("zero-value lifecycle state is ready")
	}
	var state *Readiness
	if state.Ready() {
		t.Fatal("nil lifecycle state is ready")
	}
}

func TestServeHTTPTracksReadyAndDraining(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	state := &Readiness{}
	done := make(chan error, 1)
	go func() {
		done <- ServeHTTP(ctx, &http.Server{
			Addr:              "127.0.0.1:0",
			Handler:           http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
			ReadHeaderTimeout: time.Second,
		}, state)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for !state.Ready() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !state.Ready() {
		cancel()
		t.Fatal("server never became ready")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve() = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not stop after cancellation")
	}
	if state.Ready() {
		t.Fatal("server remains ready after shutdown")
	}
}

func TestServeHTTPRejectsInvalidDependencies(t *testing.T) {
	t.Parallel()
	if err := ServeHTTP(context.Background(), nil, &Readiness{}); err == nil {
		t.Fatal("ServeHTTP(nil server) succeeded")
	}
	if err := ServeHTTP(context.Background(), &http.Server{}, nil); err == nil {
		t.Fatal("ServeHTTP(nil state) succeeded")
	}
}
