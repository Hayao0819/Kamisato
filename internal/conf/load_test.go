package conf

import "testing"

func TestLoadConfigEnvResolvesTagPaths(t *testing.T) {
	t.Setenv("AYATO_AUTH_SESSION_SECRET", testSessionSecret)
	t.Setenv("AYATO_MIKO_URL", "http://miko:8081")
	t.Setenv("AYATO_REQUIRE_SIGN", "true")

	cfg, err := Load[AyatoConfig](nil, nil, nil, "AYATO")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := cfg.Auth.SessionSecret; len(got) != 1 || got[0] != testSessionSecret {
		t.Errorf("Auth.SessionSecret = %v, want [%q]", got, testSessionSecret)
	}
	if cfg.Miko.URL != "http://miko:8081" {
		t.Errorf("Miko.URL = %q, want %q", cfg.Miko.URL, "http://miko:8081")
	}
	if !cfg.RequireSign {
		t.Errorf("RequireSign = false, want true")
	}
}
