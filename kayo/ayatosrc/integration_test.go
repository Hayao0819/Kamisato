package ayatosrc

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	ayatoaur "github.com/Hayao0819/Kamisato/ayato/aur"
	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
)

// TestAgainstRealAyatoHandler drives kayo's Source against ayato's actual gin
// handlers (real backend, real signer, gin's own JSON rendering), so the signed
// envelope and the verifier agree on the exact wire bytes — not just a stub.
func TestAgainstRealAyatoHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	seed, err := ayatoaur.GenerateSeed()
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ayatoaur.NewCatalogSigner(seed, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	svc := ayatoaur.NewService(ayatoaur.NewBackend(store, "maint"), time.Hour).WithSigner(signer)
	h := handler.NewAURHandler(svc)
	r := gin.New()
	r.GET(catalogPath, h.CatalogHandler)
	r.GET(pubkeyPath, h.PubkeyHandler)
	ts := httptest.NewServer(r)
	defer ts.Close()

	src, err := New(Options{Name: "real", BaseURL: ts.URL, PubKey: signer.PublicKeyB64(), MaxAge: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Sync(context.Background()); err != nil {
		t.Fatalf("Sync against real handler: %v", err)
	}
	if !src.Verified() {
		t.Error("a catalog signed by ayato's real handler should verify under the pinned key")
	}
}
