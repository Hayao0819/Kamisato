// Package signer is miko's optional remote-signing tier. The RemoteSigner is the
// worker-side client that offloads package signing to a dedicated signer service,
// and Handler is that service: it holds the private key and returns detached
// signatures, so build workers can run keyless. RemoteSigner implements
// pkg/pacman/sign.Signer, so the worker's publish path is unchanged.
package signer

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/auth/apikey"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/httpx"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

// SignPath is the signer service's detach-sign endpoint, shared by client and
// server so the two cannot drift.
const SignPath = "/api/unstable/sign"

const maxSignatureBytes = 16 << 20

// RemoteSigner POSTs a built package to the signer service and writes the returned
// detached signature next to it, so the build worker holds no private key.
type RemoteSigner struct {
	url    string
	apiKey string
	client *http.Client
}

var _ sign.Signer = (*RemoteSigner)(nil)

// NewRemoteSigner returns a Signer that calls the signer service at baseURL,
// authenticating with apiKey.
func NewRemoteSigner(baseURL, apiKey string) *RemoteSigner {
	return &RemoteSigner{
		url:    strings.TrimRight(baseURL, "/") + SignPath,
		apiKey: apiKey,
		// Signing a large artifact can take a while; a generous per-attempt timeout
		// with a couple of retries rides out a transient network blip.
		client: httpx.New(2*time.Minute, 2),
	}
}

func (s *RemoteSigner) Sign(ctx context.Context, pkgPath string) (string, error) {
	pkg, err := os.Open(pkgPath)
	if err != nil {
		return "", errors.WrapErr(err, "remote signer: open package")
	}
	defer func() { _ = pkg.Close() }()
	info, err := pkg.Stat()
	if err != nil {
		return "", errors.WrapErr(err, "remote signer: stat package")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, pkg)
	if err != nil {
		return "", err
	}
	req.ContentLength = info.Size()
	// retryablehttp can replay the request without retaining a potentially huge
	// package in memory; every retry gets an independent file descriptor.
	req.GetBody = func() (io.ReadCloser, error) { return os.Open(pkgPath) }
	req.Header.Set("Content-Type", "application/octet-stream")
	if s.apiKey != "" {
		req.Header.Set(apikey.Header, s.apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", errors.WrapErr(err, "remote signer: request")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", errors.NewErrf("remote signer: status %d: %s", resp.StatusCode, string(body))
	}
	if resp.ContentLength > maxSignatureBytes {
		return "", errors.NewErr("remote signer: signature response too large")
	}

	sig, err := io.ReadAll(io.LimitReader(resp.Body, maxSignatureBytes+1))
	if err != nil {
		return "", errors.WrapErr(err, "remote signer: read signature")
	}
	if len(sig) > maxSignatureBytes {
		return "", errors.NewErr("remote signer: signature response too large")
	}
	// Fail closed: an empty response is not a signature, so refuse it rather than
	// write a zero-byte .sig that would masquerade as signed.
	if len(sig) == 0 {
		return "", errors.NewErr("remote signer: empty signature")
	}
	sigPath := pkgPath + ".sig"
	// The signature is transient worker state consumed by the uploader; 0600 is
	// sufficient and keeps the at-rest footprint minimal.
	if err := os.WriteFile(sigPath, sig, 0o600); err != nil {
		return "", errors.WrapErr(err, "remote signer: write signature")
	}
	return sigPath, nil
}
