// Package lifecycle owns Ayato's process readiness and graceful HTTP server
// lifecycle. It deliberately has no dependency on Gin, repositories, or config,
// so transport wiring can observe readiness without owning process state.
package lifecycle

import (
	"context"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

const shutdownTimeout = 15 * time.Second

// State is the process-level readiness state. Its zero value is unready: an
// instance becomes ready only after initialization and listener creation have
// both succeeded.
type State struct {
	ready atomic.Bool
}

func (s *State) Ready() bool {
	return s != nil && s.ready.Load()
}

func (s *State) setReady(ready bool) {
	if s != nil {
		s.ready.Store(ready)
	}
}

// Serve listens, marks the process ready, and serves until ctx is cancelled or
// the HTTP server fails. Cancellation first makes the process unready, then
// waits for in-flight requests before forcibly closing on the deadline.
func Serve(ctx context.Context, server *http.Server, state *State) error {
	if server == nil {
		return errors.New("lifecycle: nil HTTP server")
	}
	if state == nil {
		return errors.New("lifecycle: nil readiness state")
	}

	var listenerConfig net.ListenConfig
	listener, err := listenerConfig.Listen(ctx, "tcp", server.Addr)
	if err != nil {
		return errors.WrapErr(err, "listen for HTTP requests")
	}

	serveErr := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serveErr <- err
	}()
	state.setReady(true)

	select {
	case err := <-serveErr:
		state.setReady(false)
		return errors.WrapErr(err, "HTTP server stopped")
	case <-ctx.Done():
		state.setReady(false)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	shutdownErr := server.Shutdown(shutdownCtx)
	if shutdownErr != nil {
		// Shutdown leaves active connections open when its deadline expires.
		// Close establishes a hard upper bound and makes Serve return.
		shutdownErr = errors.Join(shutdownErr, server.Close())
	}
	serveFailure := <-serveErr
	return errors.Join(
		errors.WrapErr(shutdownErr, "graceful HTTP shutdown failed"),
		errors.WrapErr(serveFailure, "HTTP server stopped during shutdown"),
	)
}
