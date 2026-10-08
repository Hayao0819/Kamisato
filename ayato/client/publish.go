package client

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	httpclient "github.com/Hayao0819/Kamisato/internal/http/client"
)

func (c *Publisher) RemovePackage(ctx context.Context, repo, arch, name string) error {
	return removePackage(ctx, c.request, repo, arch, name)
}

func removePackage(ctx context.Context, requester *httpclient.Requester, repo, arch, name string) error {
	return requester.Execute(ctx, func() error {
		return requester.Transport().DoJSON(
			ctx,
			httpclient.NoRetry,
			http.MethodDelete,
			requester.Transport().Endpoint("api", "unstable", "repos", repo, arch, "packages", name),
			true,
			nil,
			nil,
			http.StatusOK,
			"remove package",
		)
	})
}

// RemovePackageAllArchitectures removes a package from every architecture.
func (c *Client) RemovePackageAllArchitectures(ctx context.Context, repo, name string) error {
	return removePackageAllArchitectures(ctx, c.request, repo, name)
}

func (c *Publisher) RemovePackageAllArchitectures(ctx context.Context, repo, name string) error {
	return removePackageAllArchitectures(ctx, c.request, repo, name)
}

func removePackageAllArchitectures(ctx context.Context, requester *httpclient.Requester, repo, name string) error {
	return requester.Execute(ctx, func() error {
		return requester.Transport().DoJSON(
			ctx,
			httpclient.NoRetry,
			http.MethodDelete,
			requester.Transport().Endpoint("api", "unstable", "repos", repo, "packages", name),
			true,
			nil,
			nil,
			http.StatusOK,
			"remove package from all architectures",
		)
	})
}

func (c *Publisher) RegisterSigner(ctx context.Context, armoredPublicKey []byte) (string, error) {
	var fingerprint string
	err := c.request.Execute(ctx, func() error {
		attemptCtx, cancel := c.request.Transport().AttemptContext(ctx)
		defer cancel()
		req, err := c.request.Transport().NewRequest(
			attemptCtx,
			http.MethodPost,
			c.request.Transport().Endpoint("api", "unstable", "auth", "signers"),
			bytes.NewReader(armoredPublicKey),
			true,
		)
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/pgp-keys")
		resp, err := c.request.Transport().Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return httpclient.ReadResponseError(resp, "register signer")
		}
		var result struct {
			Fingerprint string `json:"fingerprint"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return err
		}
		fingerprint = result.Fingerprint
		return nil
	})
	return fingerprint, err
}
