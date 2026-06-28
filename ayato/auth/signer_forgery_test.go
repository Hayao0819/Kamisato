package auth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const rtSecret = "0123456789abcdef0123456789abcdef" // 32 bytes

// TestRedteam_ForgeWithoutSecret: an attacker must not be able to mint a valid
// token without the secret.
func TestRedteam_ForgeWithoutSecret(t *testing.T) {
	s, err := NewSigner([]string{rtSecret})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	payload := Claims{Typ: TypSession, GitHubID: 1337, Login: "attacker", Exp: time.Now().Add(time.Hour)}
	pj, _ := json.Marshal(payload)
	pB64 := base64.RawURLEncoding.EncodeToString(pj)

	forgeries := map[string]string{
		"empty-sig":          pB64 + ".",
		"empty-sig-no-dot":   pB64,
		"zero-byte-sig":      pB64 + "." + base64.RawURLEncoding.EncodeToString(make([]byte, 32)),
		"garbage-sig":        pB64 + "." + base64.RawURLEncoding.EncodeToString([]byte("not the real mac at all yo")),
		"hmac-empty-key":     pB64 + "." + base64.RawURLEncoding.EncodeToString(mac([]byte(""), pB64)),
		"hmac-payload-askey": pB64 + "." + base64.RawURLEncoding.EncodeToString(mac([]byte(pB64), pB64)),
	}
	for name, tok := range forgeries {
		if _, err := s.Verify(tok); err == nil {
			t.Fatalf("FORGERY ACCEPTED (%s): attacker minted a valid session without the secret", name)
		}
	}
}

// TestRedteam_NoExpIsRejected: a token with no Exp (zero time) must be treated as
// already expired (fail-closed), not as never-expiring.
func TestRedteam_NoExpIsRejected(t *testing.T) {
	s, _ := NewSigner([]string{rtSecret})
	tok, err := s.Sign(Claims{Typ: TypSession, GitHubID: 1, Login: "x"})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if _, err := s.Verify(tok); err == nil {
		t.Fatal("a token with no/zero Exp must be rejected (fail-closed), but Verify accepted it")
	}
}

// TestRedteam_CrossTypeReplayMatrix: VerifyTyp must reject a token minted as a
// different type even with a valid signature, blocking cross-type replay.
func TestRedteam_CrossTypeReplayMatrix(t *testing.T) {
	s, _ := NewSigner([]string{rtSecret})
	types := []string{TypSession, TypCLI, TypCode, TypState}
	for _, minted := range types {
		tok, _ := s.Sign(Claims{Typ: minted, GitHubID: 9, Exp: time.Now().Add(time.Hour)})
		for _, want := range types {
			_, err := s.VerifyTyp(tok, want)
			if minted == want && err != nil {
				t.Fatalf("same-type %q must verify: %v", minted, err)
			}
			if minted != want && err == nil {
				t.Fatalf("CROSS-TYPE REPLAY: a %q token passed VerifyTyp(%q)", minted, want)
			}
		}
	}
}

// TestRedteam_SignatureMalleability: padding, case, or trailing-byte tweaks on the
// signature segment must not defeat verification.
func TestRedteam_SignatureMalleability(t *testing.T) {
	s, _ := NewSigner([]string{rtSecret})
	tok, _ := s.Sign(Claims{Typ: TypSession, GitHubID: 5, Exp: time.Now().Add(time.Hour)})
	payload, sig, _ := strings.Cut(tok, ".")

	for _, mut := range []string{
		payload + "." + sig + "=",
		payload + "." + sig + "A",
		payload + "." + strings.ToUpper(sig),
		payload + "." + sig + ".",
		payload + "==." + sig,
	} {
		if mut == tok {
			continue
		}
		if _, err := s.Verify(mut); err == nil {
			t.Fatalf("malleable token accepted: %q", mut)
		}
	}
	if _, err := s.Verify(tok); err != nil {
		t.Fatalf("baseline token must verify: %v", err)
	}
}
