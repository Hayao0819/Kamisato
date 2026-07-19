package client

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

const (
	CatalogPath          = "/api/unstable/aur/catalog"
	CatalogPublicKeyPath = "/api/unstable/aur/pubkey"
	maxCatalogBytes      = 32 << 20
	maxPublicKeyBytes    = 64 << 10
)

// Catalog is the public read-only Ayato client.
type Catalog struct{ transport *transport }

func NewCatalog(base string, opts ...Option) (*Catalog, error) {
	transport, err := newTransport(base, noCredential{}, opts...)
	if err != nil {
		return nil, err
	}
	return &Catalog{transport: transport}, nil
}

func (c *Catalog) Fetch(ctx context.Context) ([]byte, error) {
	return c.transport.readBytes(
		ctx,
		c.transport.endpoint("api", "unstable", "aur", "catalog"),
		maxCatalogBytes,
		"fetch Ayato catalog",
	)
}

func (c *Catalog) FetchPublicKey(ctx context.Context) ([]byte, error) {
	return c.transport.readBytes(
		ctx,
		c.transport.endpoint("api", "unstable", "aur", "pubkey"),
		maxPublicKeyBytes,
		"fetch Ayato catalog public key",
	)
}

func (t *transport) readBytes(ctx context.Context, targetURL *url.URL, maxBytes int64, operation string) ([]byte, error) {
	for attempt := 0; attempt < t.readAttempts; attempt++ {
		attemptCtx := ctx
		cancel := func() {}
		if t.attemptTimeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, t.attemptTimeout)
		}
		request, err := t.newRequest(attemptCtx, http.MethodGet, targetURL, nil, false)
		if err != nil {
			cancel()
			return nil, errors.WrapErr(err, "create "+operation+" request")
		}
		request.Header.Set("Accept", "application/json")
		response, err := t.http.Do(request)
		if err != nil {
			cancel()
			if attempt+1 < t.readAttempts && ctx.Err() == nil {
				if err := retryDelay(ctx, attempt); err != nil {
					return nil, err
				}
				continue
			}
			return nil, errors.WrapErr(err, operation)
		}
		if response.StatusCode != http.StatusOK {
			if attempt+1 < t.readAttempts && retryableStatus(response.StatusCode) {
				_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
				_ = response.Body.Close()
				cancel()
				if err := retryDelay(ctx, attempt); err != nil {
					return nil, err
				}
				continue
			}
			err := responseError(response, operation)
			cancel()
			return nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
		_ = response.Body.Close()
		cancel()
		if readErr != nil {
			return nil, errors.WrapErr(readErr, operation)
		}
		if int64(len(body)) > maxBytes {
			return nil, errors.NewErrf("%s response exceeds %d bytes", operation, maxBytes)
		}
		return body, nil
	}
	return nil, errors.NewErr("unreachable catalog retry state")
}
