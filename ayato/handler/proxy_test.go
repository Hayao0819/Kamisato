package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/conf"
)

// TestMikoProxyStripsAuthorization verifies the Director removes a client's
// Authorization header (and X-API-Key) before forwarding to miko, so a user's
// CLI Bearer token never leaks to the build server.
func TestMikoProxyStripsAuthorization(t *testing.T) {
	cfg := &conf.AyatoConfig{}
	cfg.Miko.URL = "http://miko.internal:8081"
	cfg.Miko.APIKey = "shared-secret"

	mp, err := NewMikoProxy(cfg)
	if err != nil {
		t.Fatalf("NewMikoProxy: %v", err)
	}
	if mp == nil {
		t.Fatalf("expected a proxy, got nil")
	}

	req := httptest.NewRequest(http.MethodPost, "http://ayato/api/unstable/build", nil)
	req.Header.Set("Authorization", "Bearer user-cli-token")
	req.Header.Set("X-API-Key", "client-supplied-key")
	req.Header.Set("Cookie", "ayato_session=secret-sid")

	mp.proxy.Director(req)

	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization must be stripped, got %q", got)
	}
	if got := req.Header.Get("Cookie"); got != "" {
		t.Fatalf("Cookie must be stripped (no user session into miko), got %q", got)
	}
	if got := req.Header.Get("X-API-Key"); got != "shared-secret" {
		t.Fatalf("X-API-Key must be the shared secret, got %q", got)
	}
}
