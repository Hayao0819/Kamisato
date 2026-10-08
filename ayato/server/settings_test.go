package server

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/config"
	"github.com/Hayao0819/Kamisato/ayato/service"
)

func TestServiceSettingsLoadsConfiguredPublicKeyMaterial(t *testing.T) {
	path := filepath.Join(t.TempDir(), "verification.gpg")
	material := []byte("public-key-input")
	if err := os.WriteFile(path, material, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.AyatoConfig{RequireSign: true}
	cfg.Verify.Keyring = path
	settings, err := serviceSettings(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !settings.RequireSign || !bytes.Equal(settings.VerificationKeys, material) {
		t.Fatalf("verification policy/material = %+v", settings)
	}
	// Reading the configured file is assembly's responsibility; rejecting its
	// malformed contents is the service's fail-closed trust policy.
	if err := service.New(nil, nil, nil, nil, settings).InitAll(); err == nil {
		t.Fatal("malformed verification material was accepted")
	}
	cfg.Verify.Keyring = filepath.Join(t.TempDir(), "missing.gpg")
	if _, err := serviceSettings(cfg); err == nil {
		t.Fatal("missing configured verification keyring was ignored")
	}
}

func TestServiceSettingsPreservesConfiguredEmptyKeyring(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.gpg")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.AyatoConfig{}
	cfg.Verify.Keyring = path
	settings, err := serviceSettings(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if settings.VerificationKeys == nil {
		t.Fatal("configured empty keyring was confused with no keyring")
	}
	if err := service.New(nil, nil, nil, nil, settings).InitAll(); err == nil {
		t.Fatal("configured empty keyring silently disabled verification")
	}
}
