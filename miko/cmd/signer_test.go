package cmd

import (
	"context"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/signer"
)

func TestBuildSignerModes(t *testing.T) {
	ctx := context.Background()

	// Default is disabled even when a data directory exists.
	s, err := buildSigner(ctx, &conf.MikoConfig{})
	if err != nil {
		t.Fatalf("disabled default: %v", err)
	}
	if s != nil {
		t.Fatalf("default mode must leave signing disabled, got %T", s)
	}
	disabledWithData := &conf.MikoConfig{DataDir: t.TempDir()}
	if s, err = buildSigner(ctx, disabledWithData); err != nil || s != nil {
		t.Fatalf("data_dir must not implicitly enable signing: signer=%T err=%v", s, err)
	}

	// Host signing is available, but only by explicit opt-in.
	local := &conf.MikoConfig{DataDir: t.TempDir()}
	local.Signing.Mode = "local"
	if s, err = buildSigner(ctx, local); err != nil || s == nil {
		t.Fatalf("explicit local mode: signer=%T err=%v", s, err)
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
