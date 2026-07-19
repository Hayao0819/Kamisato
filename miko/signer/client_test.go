package signer_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/auth/apikey"
	"github.com/Hayao0819/Kamisato/miko/signer"
)

func newRemoteSigner(t *testing.T, base, key string) *signer.RemoteSigner {
	t.Helper()
	remote, err := signer.NewRemoteSigner(base, key)
	if err != nil {
		t.Fatalf("NewRemoteSigner: %v", err)
	}
	return remote
}

// The client must send the artifact bytes with its API key and write the returned
// signature next to the package.
func TestRemoteSignerSendsArtifactAndAppliesSignature(t *testing.T) {
	const key = "worker-key"
	var gotBody []byte
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get(apikey.Header)
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte("SIGNATURE-BYTES"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	pkgPath := filepath.Join(dir, "foo-1.0-1-x86_64.pkg.tar.zst")
	if err := os.WriteFile(pkgPath, []byte("package payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	sigPath, err := newRemoteSigner(t, srv.URL, key).Sign(context.Background(), pkgPath)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if gotAuth != key {
		t.Fatalf("signer got auth %q, want %q", gotAuth, key)
	}
	if string(gotBody) != "package payload" {
		t.Fatalf("signer got body %q, want the package bytes", gotBody)
	}
	if sigPath != pkgPath+".sig" {
		t.Fatalf("sigPath = %q, want %q", sigPath, pkgPath+".sig")
	}
	sig, err := os.ReadFile(sigPath)
	if err != nil {
		t.Fatalf("read sig: %v", err)
	}
	if string(sig) != "SIGNATURE-BYTES" {
		t.Fatalf("signature = %q, want the returned bytes", sig)
	}
}

// An unauthorized signer response must fail the client and leave no .sig behind
// (fail closed: never proceed as if signed).
func TestRemoteSignerRejectsUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(apikey.Header) != "correct" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte("sig"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	pkgPath := filepath.Join(dir, "bar.pkg.tar.zst")
	if err := os.WriteFile(pkgPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := newRemoteSigner(t, srv.URL, "wrong").Sign(context.Background(), pkgPath); err == nil {
		t.Fatal("an unauthorized signer response must fail")
	}
	if _, err := os.Stat(pkgPath + ".sig"); !os.IsNotExist(err) {
		t.Fatal("no signature must be written when signing is rejected")
	}
}

// An empty 200 body is not a signature; the client must refuse it rather than
// write a zero-byte .sig.
func TestRemoteSignerRejectsEmptySignature(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	dir := t.TempDir()
	pkgPath := filepath.Join(dir, "baz.pkg.tar.zst")
	if err := os.WriteFile(pkgPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := newRemoteSigner(t, srv.URL, "k").Sign(context.Background(), pkgPath); err == nil {
		t.Fatal("an empty signature must be rejected")
	}
	if _, err := os.Stat(pkgPath + ".sig"); !os.IsNotExist(err) {
		t.Fatal("no signature file must be written for an empty response")
	}
}

func TestRemoteSignerRejectsOversizedSignature(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "16777217")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	pkgPath := filepath.Join(t.TempDir(), "pkg.tar.zst")
	if err := os.WriteFile(pkgPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := newRemoteSigner(t, srv.URL, "k").Sign(context.Background(), pkgPath); err == nil {
		t.Fatal("oversized signature response must be rejected")
	}
}
