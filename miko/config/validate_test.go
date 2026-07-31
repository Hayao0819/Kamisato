package config

import (
	"testing"

	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
)

func TestMikoValidate(t *testing.T) {
	for _, ok := range []string{"", "container", "chroot"} {
		if err := (&MikoConfig{Executor: ok}).Validate(); err != nil {
			t.Errorf("executor %q should be valid: %v", ok, err)
		}
	}
	if err := (&MikoConfig{Builder: builder.HostConfig{
		Backend: builder.KindBwrap,
		Bwrap:   builder.BwrapConfig{Rootfs: "/srv/arch-rootfs"},
	}}).Validate(); err != nil {
		t.Errorf("configured bwrap executor should be valid: %v", err)
	}
	if err := (&MikoConfig{Executor: "bwrap"}).Validate(); err == nil {
		t.Error("bwrap without builder.bwrap.rootfs should be rejected")
	}
	if err := (&MikoConfig{Executor: "docker"}).Validate(); err == nil {
		t.Error("expected an error for an unknown executor")
	}
	withAyato := &MikoConfig{}
	withAyato.Ayato.URL = "https://ayato.example/prefix"
	if err := withAyato.Validate(); err == nil {
		t.Error("ayato.url without an X-API-Key must fail")
	}
	withAyato.Ayato.Username = "legacy"
	withAyato.Ayato.Password = "basic-secret"
	if err := withAyato.Validate(); err == nil {
		t.Error("legacy Ayato Basic credentials must fail")
	}
	withAyato.Ayato.APIKey = "publisher-key"
	if err := withAyato.Validate(); err != nil {
		t.Errorf("Ayato URL with service API key rejected: %v", err)
	}
}

func TestMikoValidateAllowsRotationWithStablePrincipal(t *testing.T) {
	cfg := &MikoConfig{Auth: MikoAuthConfig{APIKeys: []MikoAPIKey{
		{Name: "thoma-2026-01", Principal: "thoma", Key: "old", Scopes: []string{"build:read"}},
		{Name: "thoma-2026-07", Principal: "thoma", Key: "new", Scopes: []string{"build:read"}},
	}}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("rotation entries with one stable principal rejected: %v", err)
	}
}

func TestMikoValidateRejectsNegativeLogLimits(t *testing.T) {
	for _, cfg := range []*MikoConfig{
		{MaxLogBytes: -1},
		{MaxLogReaders: -1},
	} {
		if err := cfg.Validate(); err == nil {
			t.Errorf("Validate(%+v) succeeded, want error", cfg)
		}
	}
}
