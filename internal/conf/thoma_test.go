package conf

import "testing"

func TestThomaValidate(t *testing.T) {
	if err := (&ThomaConfig{}).Validate(); err == nil {
		t.Error("expected an error for an empty repo")
	}
	if err := (&ThomaConfig{Repo: "aur"}).Validate(); err != nil {
		t.Errorf("default mode rejected: %v", err)
	}
	if err := (&ThomaConfig{Repo: "aur", Mode: ThomaModeAyato}).Validate(); err != nil {
		t.Errorf("ayato mode rejected: %v", err)
	}
	if err := (&ThomaConfig{Repo: "aur", Mode: "local"}).Validate(); err == nil {
		t.Error("expected an error for an unknown mode")
	}
	if err := (&ThomaConfig{Repo: "aur", Mode: ThomaModeDirect}).Validate(); err == nil {
		t.Error("expected an error for direct mode without an api key")
	}
	if err := (&ThomaConfig{Repo: "aur", Mode: ThomaModeDirect, ApiKey: "k"}).Validate(); err != nil {
		t.Errorf("direct mode with api key rejected: %v", err)
	}
}

func TestLoadThomaConfigEnvParsesTimeout(t *testing.T) {
	t.Setenv("THOMA_REPO", "aur")
	t.Setenv("THOMA_TIMEOUT", "15")
	t.Setenv("THOMA_MODE", ThomaModeDirect)
	t.Setenv("THOMA_API_KEY", "secret")

	cfg, err := Load[ThomaConfig](nil, nil, nil, "THOMA")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Repo != "aur" || cfg.Timeout != 15 || cfg.Mode != ThomaModeDirect || cfg.ApiKey != "secret" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

// A non-numeric timeout must surface as an error, not be swallowed the way the
// old strconv.Atoi(_, _) did.
func TestLoadThomaConfigRejectsBadTimeout(t *testing.T) {
	t.Setenv("THOMA_TIMEOUT", "soon")
	if _, err := Load[ThomaConfig](nil, nil, nil, "THOMA"); err == nil {
		t.Error("expected a parse error for a non-numeric timeout")
	}
}
