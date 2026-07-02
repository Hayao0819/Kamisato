package conf

import "testing"

func TestLumineValidate(t *testing.T) {
	if err := (&LumineConfig{AuthMode: "cookie"}).Validate(); err != nil {
		t.Errorf("cookie mode rejected: %v", err)
	}
	if err := (&LumineConfig{AuthMode: "bearer", AyatoURL: "http://ayato:8080"}).Validate(); err != nil {
		t.Errorf("bearer with ayato_url rejected: %v", err)
	}
	if err := (&LumineConfig{AuthMode: "bearer"}).Validate(); err == nil {
		t.Error("expected an error for bearer mode without ayato_url")
	}
	if err := (&LumineConfig{AuthMode: "basic"}).Validate(); err == nil {
		t.Error("expected an error for an unknown auth mode")
	}
}

// TestLoadLumineConfigDefaultsAndEnv checks the defaults hook fills addr/auth_mode
// and that LUMINE_AYATO_URL resolves through koanf (the old hand-rolled fallback).
func TestLoadLumineConfigDefaultsAndEnv(t *testing.T) {
	t.Setenv("LUMINE_AYATO_URL", "http://ayato:8080")

	cfg, err := LoadLumineConfig(nil, "")
	if err != nil {
		t.Fatalf("LoadLumineConfig: %v", err)
	}
	if cfg.Addr != ":8080" {
		t.Errorf("Addr = %q, want the default \":8080\"", cfg.Addr)
	}
	if cfg.AuthMode != "cookie" {
		t.Errorf("AuthMode = %q, want the default \"cookie\"", cfg.AuthMode)
	}
	if cfg.AyatoURL != "http://ayato:8080" {
		t.Errorf("AyatoURL = %q, want the env value", cfg.AyatoURL)
	}
}
