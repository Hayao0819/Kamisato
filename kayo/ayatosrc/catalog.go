package ayatosrc

import (
	"context"

	"github.com/Hayao0819/Kamisato/internal/apiclient"
)

const (
	catalogPath          = "/api/unstable/aur/catalog"
	catalogPublicKeyPath = "/api/unstable/aur/pubkey"
	maxCatalogBytes      = 32 << 20
	maxPublicKeyBytes    = 64 << 10
)

type catalogClient struct {
	transport *apiclient.Transport
}

func newCatalogClient(base string, opts ...apiclient.Option) (*catalogClient, error) {
	transport, err := apiclient.NewPublicTransport(base, opts...)
	if err != nil {
		return nil, err
	}
	return &catalogClient{transport: transport}, nil
}

func (c *catalogClient) fetch(ctx context.Context) ([]byte, error) {
	return c.transport.ReadBytes(
		ctx,
		c.transport.Endpoint("api", "unstable", "aur", "catalog"),
		maxCatalogBytes,
		"fetch Ayato catalog",
		"application/json",
	)
}

func (c *catalogClient) fetchPublicKey(ctx context.Context) ([]byte, error) {
	return c.transport.ReadBytes(
		ctx,
		c.transport.Endpoint("api", "unstable", "aur", "pubkey"),
		maxPublicKeyBytes,
		"fetch Ayato catalog public key",
		"application/octet-stream",
	)
}
