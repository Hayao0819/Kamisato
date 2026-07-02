package ayatoclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRevokeCLIToken(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	if err := RevokeCLIToken(context.Background(), srv.URL, "tok-123"); err != nil {
		t.Fatalf("RevokeCLIToken: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/unstable/auth/cli/revoke" {
		t.Errorf("path = %q, want /api/unstable/auth/cli/revoke", gotPath)
	}
	if gotAuth != "Bearer tok-123" {
		t.Errorf("auth = %q, want Bearer tok-123", gotAuth)
	}
}

func TestRevokeCLITokenServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	if err := RevokeCLIToken(context.Background(), srv.URL, "tok"); err == nil {
		t.Fatal("expected an error on a non-200 response")
	}
}
