package conf

import (
	"reflect"
	"testing"
)

// TestAyatoAPIKeysFromEnvOnly proves the real AyatoConfig accepts its struct-slice
// auth.ci.api_keys (name/key/publish_repos) from a single env var, and a simple
// string slice (trusted_proxies) from a comma list — the Cloud Run scenario.
func TestAyatoAPIKeysFromEnvOnly(t *testing.T) {
	t.Setenv("AYATO_AUTH_CI_API_KEYS", `[{"name":"ci","key":"s3cr3t","publish_repos":["core"]}]`)
	t.Setenv("AYATO_AUTH_TRUSTED_PROXIES", "10.0.0.0/8,192.168.0.0/16")

	cfg, err := Load[AyatoConfig](nil, nil, nil, "AYATO")
	if err != nil {
		t.Fatal(err)
	}
	want := []CIAPIKey{{Name: "ci", Key: "s3cr3t", PublishRepos: []string{"core"}}}
	if !reflect.DeepEqual(cfg.Auth.CI.APIKeys, want) {
		t.Fatalf("api_keys = %+v, want %+v", cfg.Auth.CI.APIKeys, want)
	}
	if !reflect.DeepEqual(cfg.Auth.TrustedProxies, []string{"10.0.0.0/8", "192.168.0.0/16"}) {
		t.Fatalf("trusted_proxies = %v", cfg.Auth.TrustedProxies)
	}
	if err := cfg.Auth.CI.validate(); err != nil {
		t.Fatalf("env-configured api_keys failed validation: %v", err)
	}
}

func TestResolvePort(t *testing.T) {
	tests := []struct {
		name       string
		configured int
		portEnv    string
		want       int
	}{
		{"explicit config wins over PORT", 9000, "8080", 9000},
		{"explicit config wins with no PORT", 9000, "", 9000},
		{"PORT used when config unset", 0, "8080", 8080},
		{"default when nothing set", 0, "", DefaultPort},
		{"default when PORT unparseable", 0, "notaport", DefaultPort},
		{"default when PORT is zero", 0, "0", DefaultPort},
		{"non-positive config falls through to PORT", -1, "8080", 8080},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolvePort(tt.configured, tt.portEnv); got != tt.want {
				t.Errorf("resolvePort(%d, %q) = %d, want %d", tt.configured, tt.portEnv, got, tt.want)
			}
		})
	}
}

// TestApplyDefaultsHonorsPortEnv exercises the wiring: with no configured port,
// the Cloud Run PORT env is picked up so the server never binds ":0".
func TestApplyDefaultsHonorsPortEnv(t *testing.T) {
	t.Setenv("PORT", "8080")
	c := &AyatoConfig{}
	c.applyDefaults()
	if c.Port != 8080 {
		t.Errorf("Port after applyDefaults = %d, want 8080", c.Port)
	}
}

// TestApplyDefaultsKeepsExplicitPort checks the explicit port is preserved even
// when PORT is present.
func TestApplyDefaultsKeepsExplicitPort(t *testing.T) {
	t.Setenv("PORT", "8080")
	c := &AyatoConfig{Port: 9000}
	c.applyDefaults()
	if c.Port != 9000 {
		t.Errorf("Port after applyDefaults = %d, want 9000", c.Port)
	}
}
