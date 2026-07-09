package signer_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/internal/auth/apikey"
	"github.com/Hayao0819/Kamisato/miko/signer"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

func newKeystore(t *testing.T) *sign.Keystore {
	t.Helper()
	ks, err := sign.OpenOrCreate(t.TempDir(), "signer", "signer@example.test", "")
	if err != nil {
		t.Fatalf("OpenOrCreate: %v", err)
	}
	return ks
}

// certFile writes the keystore's worker cert to a file so sign.LoadKeyring can load
// the trust root for verification.
func certFile(t *testing.T, ks *sign.Keystore) string {
	t.Helper()
	armored, err := ks.WorkerCertArmored()
	if err != nil {
		t.Fatalf("WorkerCertArmored: %v", err)
	}
	path := filepath.Join(t.TempDir(), "worker.pub")
	if err := os.WriteFile(path, []byte(armored), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The service signs POSTed bytes and the returned detached signature must verify
// against the signer's worker cert over the same payload.
func TestSignerServiceSignsAndVerifies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ks := newKeystore(t)
	verifier := apikey.NewVerifier([]string{"secret-key"})
	ts := httptest.NewServer(signer.Handler(sign.NewHostKeySigner(ks), verifier))
	defer ts.Close()

	payload := []byte("real package bytes")
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+signer.SignPath, bytes.NewReader(payload))
	req.Header.Set(apikey.Header, "secret-key")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	sig, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	kr, err := sign.LoadKeyring(certFile(t, ks), nil)
	if err != nil {
		t.Fatalf("load keyring: %v", err)
	}
	if _, err := kr.VerifyDetached(bytes.NewReader(payload), bytes.NewReader(sig)); err != nil {
		t.Fatalf("returned signature must verify against the signer cert: %v", err)
	}
}

// The endpoint must reject a caller without a valid API key, so an untrusted
// worker cannot obtain signatures.
func TestSignerServiceRejectsUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ks := newKeystore(t)
	verifier := apikey.NewVerifier([]string{"secret-key"})
	ts := httptest.NewServer(signer.Handler(sign.NewHostKeySigner(ks), verifier))
	defer ts.Close()

	for _, tc := range []struct{ name, key string }{
		{"missing key", ""},
		{"wrong key", "nope"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+signer.SignPath, bytes.NewReader([]byte("x")))
			if tc.key != "" {
				req.Header.Set(apikey.Header, tc.key)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", resp.StatusCode)
			}
		})
	}
}
