package client

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

func (c *Publisher) RemovePackage(ctx context.Context, repo, arch, name string) error {
	return removePackage(ctx, c.request, repo, arch, name)
}

func removePackage(ctx context.Context, requester *requester, repo, arch, name string) error {
	return requester.execute(ctx, func() error {
		return requester.transport.doJSON(
			ctx,
			noRetry,
			http.MethodDelete,
			requester.transport.endpoint("api", "unstable", "repos", repo, arch, "packages", name),
			true,
			nil,
			nil,
			http.StatusOK,
			"remove package",
		)
	})
}

// RemovePackageAllArchitectures removes a package from every architecture.
func (c *Ayato) RemovePackageAllArchitectures(ctx context.Context, repo, name string) error {
	return removePackageAllArchitectures(ctx, c.request, repo, name)
}

func (c *Publisher) RemovePackageAllArchitectures(ctx context.Context, repo, name string) error {
	return removePackageAllArchitectures(ctx, c.request, repo, name)
}

func removePackageAllArchitectures(ctx context.Context, requester *requester, repo, name string) error {
	return requester.execute(ctx, func() error {
		return requester.transport.doJSON(
			ctx,
			noRetry,
			http.MethodDelete,
			requester.transport.endpoint("api", "unstable", "repos", repo, "packages", name),
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
	err := c.request.execute(ctx, func() error {
		attemptCtx, cancel := c.request.transport.attemptContext(ctx)
		defer cancel()
		req, err := c.request.transport.newRequest(
			attemptCtx,
			http.MethodPost,
			c.request.transport.endpoint("api", "unstable", "auth", "signers"),
			bytes.NewReader(armoredPublicKey),
			true,
		)
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/pgp-keys")
		resp, err := c.request.transport.http.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return responseError(resp, "register signer")
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
