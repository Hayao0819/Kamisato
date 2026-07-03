package confloader

import (
	"reflect"
	"testing"
)

type ciKey struct {
	Name         string   `koanf:"name"`
	Key          string   `koanf:"key"`
	PublishRepos []string `koanf:"publish_repos"`
}

type sliceCfg struct {
	TrustedProxies []string `koanf:"trusted_proxies"`
	SessionSecret  []string `koanf:"session_secret"`
	Auth           struct {
		CI struct {
			APIKeys []ciKey `koanf:"api_keys"`
		} `koanf:"ci"`
	} `koanf:"auth"`
	Port int `koanf:"port"`
}

// loadWithEnv loads sliceCfg using only the given env vars (no files/flags), so a
// test drives the env provider in isolation.
func loadWithEnv(t *testing.T, env map[string]string) (*sliceCfg, error) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
	l := New[sliceCfg](".").Env("TEST")
	if err := l.Load(); err != nil {
		return nil, err
	}
	return l.Get()
}

// TestEnvStructSliceFromJSON is the headline case: a slice of structs
// (auth.ci.api_keys) set entirely from one env var holding a JSON array.
func TestEnvStructSliceFromJSON(t *testing.T) {
	cfg, err := loadWithEnv(t, map[string]string{
		"TEST_AUTH_CI_API_KEYS": `[{"name":"ci","key":"s3cr3t","publish_repos":["core","extra"]},{"name":"bot","key":"k2","publish_repos":["*"]}]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Auth.CI.APIKeys
	want := []ciKey{
		{Name: "ci", Key: "s3cr3t", PublishRepos: []string{"core", "extra"}},
		{Name: "bot", Key: "k2", PublishRepos: []string{"*"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("api_keys = %+v, want %+v", got, want)
	}
}

// TestEnvStringSliceCommaSeparated checks the ergonomic form for a simple
// []string field.
func TestEnvStringSliceCommaSeparated(t *testing.T) {
	cfg, err := loadWithEnv(t, map[string]string{
		"TEST_TRUSTED_PROXIES": "10.0.0.0/8, 192.168.0.0/16 ,172.16.0.0/12",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"10.0.0.0/8", "192.168.0.0/16", "172.16.0.0/12"}
	if !reflect.DeepEqual(cfg.TrustedProxies, want) {
		t.Errorf("trusted_proxies = %v, want %v", cfg.TrustedProxies, want)
	}
}

// TestEnvStringSliceJSON checks the JSON form still works for a []string field,
// which is the escape hatch when a value legitimately contains a comma.
func TestEnvStringSliceJSON(t *testing.T) {
	cfg, err := loadWithEnv(t, map[string]string{
		"TEST_SESSION_SECRET": `["key,with,commas","second-key"]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"key,with,commas", "second-key"}
	if !reflect.DeepEqual(cfg.SessionSecret, want) {
		t.Errorf("session_secret = %v, want %v", cfg.SessionSecret, want)
	}
}

// TestEnvScalarStillWorks guards that scalar env vars are untouched by the slice
// handling.
func TestEnvScalarStillWorks(t *testing.T) {
	cfg, err := loadWithEnv(t, map[string]string{"TEST_PORT": "9123"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 9123 {
		t.Errorf("port = %d, want 9123", cfg.Port)
	}
}

// TestEnvInvalidJSONFails checks that malformed JSON for a struct slice surfaces
// as a load error rather than silently dropping the value.
func TestEnvInvalidJSONFails(t *testing.T) {
	_, err := loadWithEnv(t, map[string]string{
		"TEST_AUTH_CI_API_KEYS": `[{"name":"ci"`, // truncated
	})
	if err == nil {
		t.Fatal("malformed JSON for a struct slice must fail, got nil")
	}
}

// TestParseEnvValueScalarPassthrough is a unit check that non-slice fields pass
// their raw string through.
func TestParseEnvValueScalarPassthrough(t *testing.T) {
	got, err := parseEnvValue(reflect.TypeOf(0), "42")
	if err != nil {
		t.Fatal(err)
	}
	if got != "42" {
		t.Errorf("scalar parse = %v, want %q", got, "42")
	}
}
