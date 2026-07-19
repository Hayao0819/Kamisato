package platform

import (
	"context"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

const shutdownTimeout = 15 * time.Second

// Readiness is the process-level readiness state. Its zero value is unready: an
// instance becomes ready only after initialization and listener creation have
// both succeeded.
type Readiness struct {
	ready atomic.Bool
}

func (s *Readiness) Ready() bool {
	return s != nil && s.ready.Load()
}

func (s *Readiness) setReady(ready bool) {
	if s != nil {
		s.ready.Store(ready)
	}
}

// ServeHTTP listens, marks the process ready, and serves until ctx is cancelled or
// the HTTP server fails. Cancellation first makes the process unready, then
// waits for in-flight requests before forcibly closing on the deadline.
func ServeHTTP(ctx context.Context, server *http.Server, state *Readiness) error {
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
