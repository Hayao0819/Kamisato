package generatecmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/auth/apikey"
	mikoconfig "github.com/Hayao0819/Kamisato/miko/config"
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

func TestAppendAPIKeyRejectsInvalidCredentialsWithoutChangingConfig(t *testing.T) {
	valid := mikoconfig.MikoAPIKey{Name: "worker", Key: "existing", Scopes: []string{apikey.ScopeBuildAdmin}}
	for _, tc := range []struct {
		name  string
		entry mikoconfig.MikoAPIKey
	}{
		{"empty name", mikoconfig.MikoAPIKey{Key: "new", Scopes: []string{apikey.ScopeBuildAdmin}}},
		{"duplicate name", mikoconfig.MikoAPIKey{Name: "worker", Key: "new", Scopes: []string{apikey.ScopeBuildAdmin}}},
		{"duplicate key", mikoconfig.MikoAPIKey{Name: "new", Key: "existing", Scopes: []string{apikey.ScopeBuildAdmin}}},
		{"unknown scope", mikoconfig.MikoAPIKey{Name: "new", Key: "new", Scopes: []string{"root"}}},
		{"empty scopes", mikoconfig.MikoAPIKey{Name: "new", Key: "new"}},
		{"blank principal", mikoconfig.MikoAPIKey{Name: "new", Principal: " ", Key: "new", Scopes: []string{apikey.ScopeBuildAdmin}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "miko.json")
			if err := appendAPIKey(path, valid); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := appendAPIKey(path, tc.entry); err == nil {
				t.Fatal("invalid API key was appended")
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("rejected append changed the existing configuration")
			}
		})
	}
}

func TestAppendAPIKeyRejectsMalformedExistingConfigWithoutDataLoss(t *testing.T) {
	for _, content := range []string{"null", `{"auth":[]}`, `{"auth":null}`, `{"auth":{"api_keys":{}}}`, `{"auth":{"api_keys":[{"name":"broken"}]}}`} {
		t.Run(content, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "miko.json")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			entry := mikoconfig.MikoAPIKey{Name: "worker", Key: "new", Scopes: []string{apikey.ScopeBuildAdmin}}
			if err := appendAPIKey(path, entry); err == nil {
				t.Fatal("invalid existing configuration was overwritten")
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != content {
				t.Fatalf("configuration changed to %q", got)
			}
		})
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
			errs <- appendAPIKey(path, mikoconfig.MikoAPIKey{
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
