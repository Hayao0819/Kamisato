package shared

import (
	"context"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/signer"
)

func TestBuildSignerModes(t *testing.T) {
	ctx := context.Background()

	// Default is disabled even when a data directory exists.
	s, err := BuildSigner(ctx, &conf.MikoConfig{})
	if err != nil {
		t.Fatalf("disabled default: %v", err)
	}
	if s != nil {
		t.Fatalf("default mode must leave signing disabled, got %T", s)
	}
	disabledWithData := &conf.MikoConfig{DataDir: t.TempDir()}
	if s, err = BuildSigner(ctx, disabledWithData); err != nil || s != nil {
		t.Fatalf("data_dir must not implicitly enable signing: signer=%T err=%v", s, err)
	}

	// Host signing is available, but only by explicit opt-in.
	local := &conf.MikoConfig{DataDir: t.TempDir()}
	local.Signing.Mode = "local"
	if s, err = BuildSigner(ctx, local); err != nil || s == nil {
		t.Fatalf("explicit local mode: signer=%T err=%v", s, err)
	}

	// Remote mode builds the remote signer client.
	remote := &conf.MikoConfig{}
	remote.Signing.Mode = "remote"
	remote.Signing.Remote.URL = "http://miko-signer:8081"
	remote.Signing.Remote.APIKey = "sign-only-key"
	s, err = BuildSigner(ctx, remote)
	if err != nil {
		t.Fatalf("remote: %v", err)
	}
	if _, ok := s.(*signer.RemoteSigner); !ok {
		t.Fatalf("remote mode must return a *signer.RemoteSigner, got %T", s)
	}

	// Remote mode without a URL fails.
	noURL := &conf.MikoConfig{}
	noURL.Signing.Mode = "remote"
	if _, err := BuildSigner(ctx, noURL); err == nil {
		t.Fatal("remote mode without a URL must fail")
	}
	noKey := &conf.MikoConfig{}
	noKey.Signing.Mode = "remote"
	noKey.Signing.Remote.URL = "http://miko-signer:8081"
	if _, err := BuildSigner(ctx, noKey); err == nil {
		t.Fatal("remote mode without an API key must fail")
	}

	// An unknown mode fails loudly.
	bad := &conf.MikoConfig{}
	bad.Signing.Mode = "bogus"
	if _, err := BuildSigner(ctx, bad); err == nil {
		t.Fatal("unknown signing mode must fail")
	}
}
