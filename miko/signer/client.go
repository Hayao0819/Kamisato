// Package signer is miko's optional remote-signing tier. The RemoteSigner is the
// worker-side client that offloads package signing to a dedicated signer service,
// and Handler is that service: it holds the private key and returns detached
// signatures, so build workers can run keyless. RemoteSigner implements
// pkg/pacman/sign.Signer, so the worker's publish path is unchanged.
package signer

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/apikey"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/internal/httpx"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

// SignPath is the signer service's detach-sign endpoint, shared by client and
// server so the two cannot drift.
const SignPath = "/api/unstable/sign"

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
	pkg, err := os.ReadFile(pkgPath)
	if err != nil {
		return "", errwrap.WrapErr(err, "remote signer: read package")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(pkg))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if s.apiKey != "" {
		req.Header.Set(apikey.Header, s.apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", errwrap.WrapErr(err, "remote signer: request")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", errwrap.NewErrf("remote signer: status %d: %s", resp.StatusCode, string(body))
	}

	sig, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errwrap.WrapErr(err, "remote signer: read signature")
	}
	// Fail closed: an empty response is not a signature, so refuse it rather than
	// write a zero-byte .sig that would masquerade as signed.
	if len(sig) == 0 {
		return "", errwrap.NewErr("remote signer: empty signature")
	}
	sigPath := pkgPath + ".sig"
	// The signature is transient worker state consumed by the uploader; 0600 is
	// sufficient and keeps the at-rest footprint minimal.
	if err := os.WriteFile(sigPath, sig, 0o600); err != nil {
		return "", errwrap.WrapErr(err, "remote signer: write signature")
	}
	return sigPath, nil
}
