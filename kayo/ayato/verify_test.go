package ayato

import (
	"testing"
	"time"

	ayatoaur "github.com/Hayao0819/Kamisato/ayato/aur"
	"github.com/Hayao0819/Kamisato/internal/kayoproto"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	seed, err := ayatoaur.GenerateSeed()
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ayatoaur.NewCatalogSigner(seed, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	v, err := NewVerifier(signer.PublicKeyB64(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	cat := kayoproto.Catalog{Packages: []aurweb.Pkg{{Name: "x", PackageBase: "x"}}}
	env, err := signer.Sign(cat)
	if err != nil {
		t.Fatal(err)
	}

	if err := v.VerifyPayload(env.Payload, env.Signature); err != nil {
		t.Fatalf("valid signature must verify: %v", err)
	}

	// Tamper one payload byte -> must fail.
	bad := append([]byte(nil), env.Payload...)
	bad[len(bad)/2] ^= 0xff
	if err := v.VerifyPayload(bad, env.Signature); err == nil {
		t.Error("tampered payload must not verify")
	}

	// A verifier with a different key must reject.
	otherSeed, _ := ayatoaur.GenerateSeed()
	otherSigner, _ := ayatoaur.NewCatalogSigner(otherSeed, time.Hour)
	otherV, _ := NewVerifier(otherSigner.PublicKeyB64(), time.Hour)
	if err := otherV.VerifyPayload(env.Payload, env.Signature); err == nil {
		t.Error("signature from a different key must not verify")
	}
}

func TestCheckFreshness(t *testing.T) {
	v, _ := NewVerifier(mustPub(t), time.Hour)
	now := time.Now()

	if err := v.CheckFreshness(now, time.Time{}, time.Time{}); err != nil {
		t.Errorf("fresh catalog rejected: %v", err)
	}
	if err := v.CheckFreshness(now.Add(-2*time.Hour), time.Time{}, time.Time{}); err == nil {
		t.Error("stale catalog (older than maxAge) must be rejected")
	}
	if err := v.CheckFreshness(now.Add(10*time.Minute), time.Time{}, time.Time{}); err == nil {
		t.Error("future-dated catalog must be rejected")
	}
	if err := v.CheckFreshness(now, now.Add(-time.Hour), time.Time{}); err == nil {
		t.Error("expired catalog must be rejected")
	}
	// Rollback: issuedAt not after the last accepted watermark.
	if err := v.CheckFreshness(now.Add(-30*time.Second), time.Time{}, now); err == nil {
		t.Error("rollback (issued <= last accepted) must be rejected")
	}
	if err := v.CheckFreshness(now, time.Time{}, now.Add(-time.Minute)); err != nil {
		t.Errorf("a newer catalog than the watermark must pass: %v", err)
	}
}

func mustPub(t *testing.T) string {
	t.Helper()
	seed, _ := ayatoaur.GenerateSeed()
	s, _ := ayatoaur.NewCatalogSigner(seed, time.Hour)
	return s.PublicKeyB64()
}
