package conf

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func testPubKey(t *testing.T) string {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(pub)
}

func kayoWith(a AyatoSource) *KayoConfig {
	// An explicit TrustStore keeps these per-source checks independent of the
	// ambient user-config-dir guard.
	return &KayoConfig{Port: 10713, TrustStore: "/tmp/kayo-test/trust.json", Ayato: []AyatoSource{a}}
}

func TestAyatoSourceValidate(t *testing.T) {
	pub := testPubKey(t)
	tests := []struct {
		name    string
		src     AyatoSource
		wantErr bool
	}{
		{"pinned", AyatoSource{Name: "a", URL: "https://x", PubKey: pub}, false},
		{"trust-on-first-use", AyatoSource{Name: "a", URL: "https://x", TrustOnFirstUse: true}, false},
		{"insecure", AyatoSource{Name: "a", URL: "https://x", Insecure: true}, false},
		{"delegate-pinned", AyatoSource{Name: "a", URL: "https://x", PubKey: pub, Trust: "delegate"}, false},

		{"unpinned-secure", AyatoSource{Name: "a", URL: "https://x"}, true},
		{"delegate-without-pin", AyatoSource{Name: "a", URL: "https://x", TrustOnFirstUse: true, Trust: "delegate"}, true},
		{"insecure-with-pin", AyatoSource{Name: "a", URL: "https://x", Insecure: true, PubKey: pub}, true},
		{"insecure-with-delegate", AyatoSource{Name: "a", URL: "https://x", Insecure: true, Trust: "delegate"}, true},
		{"bad-trust", AyatoSource{Name: "a", URL: "https://x", PubKey: pub, Trust: "bogus"}, true},
		{"short-key", AyatoSource{Name: "a", URL: "https://x", PubKey: base64.StdEncoding.EncodeToString([]byte("short"))}, true},
		{"negative-maxage", AyatoSource{Name: "a", URL: "https://x", TrustOnFirstUse: true, MaxAgeMinutes: -1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := kayoWith(tt.src).Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate = %v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestAyatoTrustStoreGuard(t *testing.T) {
	pub := testPubKey(t)
	src := AyatoSource{Name: "a", URL: "https://x", PubKey: pub}

	// No user config dir available and no explicit trust_store: federation must
	// refuse rather than drop pins into a world-writable temp path.
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	if err := (&KayoConfig{Port: 10713, Ayato: []AyatoSource{src}}).Validate(); err == nil {
		t.Error("ayato + no durable trust_store should be refused")
	}
	// An explicit trust_store satisfies the requirement.
	if err := (&KayoConfig{Port: 10713, TrustStore: "/srv/kayo/trust.json", Ayato: []AyatoSource{src}}).Validate(); err != nil {
		t.Errorf("explicit trust_store should be accepted: %v", err)
	}
	// No ayato sources: the guard does not apply.
	if err := (&KayoConfig{Port: 10713}).Validate(); err != nil {
		t.Errorf("no ayato sources should not trip the guard: %v", err)
	}
}

func TestAyatoSourceResolvedMaxAge(t *testing.T) {
	if got := (AyatoSource{}).ResolvedMaxAge(); got != defaultAyatoMaxAge {
		t.Errorf("default max age = %v, want %v", got, defaultAyatoMaxAge)
	}
	if got := (AyatoSource{MaxAgeMinutes: 30}).ResolvedMaxAge().Minutes(); got != 30 {
		t.Errorf("configured max age = %v min, want 30", got)
	}
}

func TestAyatoSourceDelegated(t *testing.T) {
	pub := testPubKey(t)
	if !(AyatoSource{PubKey: pub, Trust: "delegate"}).Delegated() {
		t.Error("pinned delegate should report Delegated")
	}
	if (AyatoSource{TrustOnFirstUse: true, Trust: "delegate"}).Delegated() {
		t.Error("trust-on-first-use (unpinned) must never delegate")
	}
	if (AyatoSource{PubKey: pub}).Delegated() {
		t.Error("review (default) must not delegate")
	}
}
