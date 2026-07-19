package lifecycle

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestStateStartsUnready(t *testing.T) {
	t.Parallel()
	if (&State{}).Ready() {
		t.Fatal("zero-value lifecycle state is ready")
	}
	var state *State
	if state.Ready() {
		t.Fatal("nil lifecycle state is ready")
	}
}

func TestServeTracksReadyAndDraining(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	state := &State{}
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, &http.Server{
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

func TestServeRejectsInvalidDependencies(t *testing.T) {
	t.Parallel()
	if err := Serve(context.Background(), nil, &State{}); err == nil {
		t.Fatal("Serve(nil server) succeeded")
	}
	if err := Serve(context.Background(), &http.Server{}, nil); err == nil {
		t.Fatal("Serve(nil state) succeeded")
	}
}
