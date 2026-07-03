package cmd

import (
	"context"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/signer"
)

func TestBuildSignerModes(t *testing.T) {
	ctx := context.Background()

	// Default (local) with no key dir: signing stays disabled, no error — the
	// current behavior, unchanged.
	s, err := buildSigner(ctx, &conf.MikoConfig{})
	if err != nil {
		t.Fatalf("local default: %v", err)
	}
	if s != nil {
		t.Fatalf("local mode with no key dir must leave signing disabled, got %T", s)
	}

	// Remote mode builds the remote signer client.
	remote := &conf.MikoConfig{}
	remote.Signing.Mode = "remote"
	remote.Signing.Remote.URL = "http://miko-signer:8081"
	s, err = buildSigner(ctx, remote)
	if err != nil {
		t.Fatalf("remote: %v", err)
	}
	if _, ok := s.(*signer.RemoteSigner); !ok {
		t.Fatalf("remote mode must return a *signer.RemoteSigner, got %T", s)
	}

	// Remote mode without a URL fails.
	noURL := &conf.MikoConfig{}
	noURL.Signing.Mode = "remote"
	if _, err := buildSigner(ctx, noURL); err == nil {
		t.Fatal("remote mode without a URL must fail")
	}

	// An unknown mode fails loudly.
	bad := &conf.MikoConfig{}
	bad.Signing.Mode = "bogus"
	if _, err := buildSigner(ctx, bad); err == nil {
		t.Fatal("unknown signing mode must fail")
	}
}
