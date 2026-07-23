package apikeycmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/auth/apikey"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

func TestGenerateAPIKeyRoundTrip(t *testing.T) {
	k, err := generateAPIKey()
	if err != nil {
		t.Fatalf("generateAPIKey: %v", err)
	}
	if !strings.HasPrefix(k, "miko_") {
		t.Errorf("key %q lacks the miko_ prefix", k)
	}

	// A freshly generated key validates against a verifier configured with it.
	v := apikey.NewVerifier([]string{k})
	if !v.Valid(k) {
		t.Error("generated key did not validate against its own verifier")
	}

	other, err := generateAPIKey()
	if err != nil {
		t.Fatalf("generateAPIKey (second): %v", err)
	}
	if k == other {
		t.Error("two generated keys collided; generation is not random")
	}
	if v.Valid(other) {
		t.Error("an unrelated key validated against the verifier")
	}
}

func TestAppendAPIKeySerializesConcurrentWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "miko.json")
	const writers = 12
	start := make(chan struct{})
	errs := make(chan error, writers)
	for i := range writers {
		go func() {
			<-start
			name := fmt.Sprintf("worker-%02d", i)
			errs <- appendAPIKey(path, conf.MikoAPIKey{
				Name:   name,
				Key:    "secret-" + name,
				Scopes: []string{apikey.ScopeBuildAdmin},
			})
		}()
	}
	close(start)
	for range writers {
		if err := <-errs; err != nil {
			t.Fatalf("appendAPIKey: %v", err)
		}
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Auth struct {
			APIKeys []struct {
				Name string `json:"name"`
			} `json:"api_keys"`
		} `json:"auth"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if got := len(cfg.Auth.APIKeys); got != writers {
		t.Fatalf("stored API keys = %d, want %d", got, writers)
	}
	seen := make(map[string]bool, writers)
	for _, key := range cfg.Auth.APIKeys {
		seen[key.Name] = true
	}
	for i := range writers {
		name := fmt.Sprintf("worker-%02d", i)
		if !seen[name] {
			t.Errorf("concurrent update lost %q", name)
		}
	}
}
