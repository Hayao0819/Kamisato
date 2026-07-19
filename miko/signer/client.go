// Package signer is miko's optional remote-signing tier. The RemoteSigner is the
// worker-side client that offloads package signing to a dedicated signer service,
// and Handler is that service: it holds the private key and returns detached
// signatures, so build workers can run keyless. RemoteSigner implements
// pkg/pacman/sign.Signer, so the worker's publish path is unchanged.
package signer

import (
	"context"
	"os"

	"github.com/Hayao0819/Kamisato/internal/client"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

// SignPath is the signer service's detach-sign endpoint, shared by client and
// server so the two cannot drift.
const SignPath = client.SignPath

// RemoteSigner POSTs a built package to the signer service and writes the returned
// detached signature next to it, so the build worker holds no private key.
type RemoteSigner struct {
	client *client.Signer
}

var _ sign.Signer = (*RemoteSigner)(nil)

// NewRemoteSigner returns a Signer that calls the signer service at baseURL,
// authenticating with apiKey.
func NewRemoteSigner(baseURL, apiKey string) (*RemoteSigner, error) {
	api, err := client.NewSigner(baseURL, apiKey)
	if err != nil {
		return nil, err
	}
	return &RemoteSigner{client: api}, nil
}

func (s *RemoteSigner) Sign(ctx context.Context, pkgPath string) (string, error) {
	sig, err := s.client.SignFile(ctx, pkgPath)
	if err != nil {
		return "", err
	}
	sigPath := pkgPath + ".sig"
	// The signature is transient worker state consumed by the uploader; 0600 is
	// sufficient and keeps the at-rest footprint minimal.
	if err := os.WriteFile(sigPath, sig, 0o600); err != nil {
		return "", errors.WrapErr(err, "remote signer: write signature")
	}
	return sigPath, nil
}
