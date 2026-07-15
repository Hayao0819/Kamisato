package ayatosrc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	ayatoaur "github.com/Hayao0819/Kamisato/ayato/service/aur"
	"github.com/Hayao0819/Kamisato/internal/kayoproto"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

var testCat = kayoproto.Catalog{
	Packages: []aurweb.Pkg{{Name: "x", PackageBase: "x", Version: "1.0-1"}},
	Sources:  map[string]string{"x": "https://git.example.com/x.git"},
}

// signedServer serves a freshly-signed envelope (re-marshaled like gin's c.JSON)
// at the catalog path and the pubkey at the pubkey path.
func signedServer(t *testing.T, signer *ayatoaur.CatalogSigner, alg string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case catalogPath:
			if alg == "none" { // legacy/unsigned envelope
				payload, _ := json.Marshal(kayoproto.SignedPayload{IssuedAt: time.Now().UTC(), Catalog: testCat})
				_ = json.NewEncoder(w).Encode(kayoproto.CatalogEnvelope{Payload: payload, Alg: "none"})
				return
			}
			env, err := signer.Sign(testCat)
			if err != nil {
				t.Error(err)
			}
			wire, _ := json.Marshal(env) // re-marshal exactly as gin's render does
			_, _ = w.Write(wire)
		case pubkeyPath:
			_ = json.NewEncoder(w).Encode(map[string]string{"pubkey": signer.PublicKeyB64(), "key_id": signer.KeyID()})
		default:
			http.NotFound(w, r)
		}
	}))
}

func newSigner(t *testing.T) *ayatoaur.CatalogSigner {
	t.Helper()
	seed, err := ayatoaur.GenerateSeed()
	if err != nil {
		t.Fatal(err)
	}
	s, err := ayatoaur.NewCatalogSigner(seed, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSourceSyncSignedAndPinned(t *testing.T) {
	signer := newSigner(t)
	ts := signedServer(t, signer, "ed25519")
	defer ts.Close()

	s, err := New(Options{Name: "t", BaseURL: ts.URL, PubKey: signer.PublicKeyB64(), MaxAge: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if !s.Verified() {
		t.Error("Verified() should be true after a good signed sync")
	}
	if info, _ := s.Info(context.Background(), []string{"x"}); len(info) != 1 || info[0].Version != "1.0-1" {
		t.Fatalf("Info = %+v", info)
	}
}

func TestSourceRejectsUnsignedWhenPinned(t *testing.T) {
	signer := newSigner(t)
	ts := signedServer(t, signer, "none") // server downgrades to unsigned
	defer ts.Close()

	s, _ := New(Options{Name: "t", BaseURL: ts.URL, PubKey: signer.PublicKeyB64(), MaxAge: time.Hour})
	if err := s.Sync(context.Background()); err == nil {
		t.Error("a pinned source must refuse an unsigned (downgrade) catalog")
	}
	if info, _ := s.Info(context.Background(), []string{"x"}); len(info) != 0 {
		t.Error("rejected catalog must not populate the index (fail-closed)")
	}
}

func TestSourceTOFU(t *testing.T) {
	signer := newSigner(t)
	ts := signedServer(t, signer, "ed25519")
	defer ts.Close()

	pins, _ := OpenPinStore(filepath.Join(t.TempDir(), "known_ayato.json"))
	s, _ := New(Options{Name: "t", BaseURL: ts.URL, TrustOnFirstUse: true, MaxAge: time.Hour, Pins: pins})
	if err := s.Sync(context.Background()); err != nil {
		t.Fatalf("TOFU first-contact sync: %v", err)
	}
	if p, ok := pins.Get("t"); !ok || p.PubKey != signer.PublicKeyB64() {
		t.Error("TOFU should pin the fetched public key")
	}
	if info, _ := s.Info(context.Background(), []string{"x"}); len(info) != 1 {
		t.Error("TOFU sync should populate the index")
	}
}

func TestVerifiedFreshnessBound(t *testing.T) {
	signer := newSigner(t)
	ts := signedServer(t, signer, "ed25519")
	defer ts.Close()

	// maxAge is short but the sync-time freshness check has a 1-minute leeway, so
	// the fresh catalog still syncs. Verified() applies maxAge with no leeway, so
	// once the served catalog ages out, delegation falls closed. The 100ms margin
	// stays comfortably above race-detector jitter on the "fresh" assertion.
	s, _ := New(Options{Name: "t", BaseURL: ts.URL, PubKey: signer.PublicKeyB64(), MaxAge: 100 * time.Millisecond})
	if err := s.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if !s.Verified() {
		t.Fatal("Verified() should be true immediately after a fresh sync")
	}
	// Poll until the vouch ages out past maxAge instead of sleeping a fixed span.
	deadline := time.Now().Add(3 * time.Second)
	for s.Verified() {
		if time.Now().After(deadline) {
			t.Fatal("Verified() did not fall closed once the served catalog passes maxAge (freeze defense)")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// The last-good index is still served (fail-closed), only delegation drops.
	if info, _ := s.Info(context.Background(), []string{"x"}); len(info) != 1 {
		t.Error("the last-good catalog should still be served after Verified() expires")
	}
}

func TestVerifiedExpiresBound(t *testing.T) {
	seed, err := ayatoaur.GenerateSeed()
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ayatoaur.NewCatalogSigner(seed, 120*time.Millisecond) // short signed TTL
	if err != nil {
		t.Fatal(err)
	}
	ts := signedServer(t, signer, "ed25519")
	defer ts.Close()

	// maxAge is generous (1h), so only the catalog's own signed ExpiresAt can
	// expire the vouch. The sync-time expiry check has a 1-minute leeway, so the
	// fresh catalog still syncs; Verified() applies ExpiresAt with no leeway.
	s, _ := New(Options{Name: "t", BaseURL: ts.URL, PubKey: signer.PublicKeyB64(), MaxAge: time.Hour})
	if err := s.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if !s.Verified() {
		t.Fatal("Verified() should be true immediately after sync")
	}
	// Poll until the signed ExpiresAt lapses instead of sleeping a fixed span.
	deadline := time.Now().Add(3 * time.Second)
	for s.Verified() {
		if time.Now().After(deadline) {
			t.Fatal("Verified() did not fall closed past the catalog's signed ExpiresAt, even within maxAge")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSourceInsecure(t *testing.T) {
	signer := newSigner(t)
	ts := signedServer(t, signer, "none")
	defer ts.Close()

	s, _ := New(Options{Name: "t", BaseURL: ts.URL, Insecure: true})
	if err := s.Sync(context.Background()); err != nil {
		t.Fatalf("insecure sync of an unsigned catalog should pass: %v", err)
	}
	if info, _ := s.Info(context.Background(), []string{"x"}); len(info) != 1 {
		t.Error("insecure sync should populate the index")
	}
}
