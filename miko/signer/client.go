package signer

import (
	"context"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/filesystem/safefile"
	"github.com/Hayao0819/Kamisato/internal/pacman/sign"
	miko "github.com/Hayao0819/Kamisato/miko/client"
)

// SignPath is the signer service's detach-sign endpoint, shared by client and
// server so the two cannot drift.
const SignPath = miko.SignPath

// RemoteSigner POSTs a built package to the signer service and writes the returned
// detached signature next to it, so the build worker holds no private key.
type RemoteSigner struct {
	client *miko.Signer
}

var _ sign.Signer = (*RemoteSigner)(nil)

// NewRemoteSigner returns a Signer that calls the signer service at baseURL,
// authenticating with apiKey.
func NewRemoteSigner(baseURL, apiKey string) (*RemoteSigner, error) {
	api, err := miko.NewSigner(baseURL, apiKey)
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
	if err := safefile.WriteFile(sigPath, sig, 0o600); err != nil {
		return "", errors.WrapErr(err, "remote signer: write signature")
	}
	return sigPath, nil
}
