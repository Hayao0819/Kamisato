package auth

import (
	"strings"
	"testing"
	"time"
)

const (
	secretA = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" // 32 bytes
	secretB = "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB" // 32 bytes
)

func TestNewSignerRejectsBadSecrets(t *testing.T) {
	if _, err := NewSigner(nil); err == nil {
		t.Fatal("empty secret list must be rejected")
	}
	if _, err := NewSigner([]string{}); err == nil {
		t.Fatal("empty secret slice must be rejected")
	}
	if _, err := NewSigner([]string{"short"}); err == nil {
		t.Fatal("secret under 32 bytes must be rejected")
	}
	if _, err := NewSigner([]string{secretA, "short"}); err == nil {
		t.Fatal("any secret under 32 bytes must be rejected")
	}
}

func TestSignVerifyRoundTrip(t *testing.T) {
	s, err := NewSigner([]string{secretA})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	in := Claims{Typ: TypSession, GitHubID: 42, Login: "alice", Exp: time.Now().Add(time.Hour)}
	tok, err := s.Sign(in)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	out, err := s.verify(tok)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if out.Typ != TypSession || out.GitHubID != 42 || out.Login != "alice" {
		t.Fatalf("round-trip claims = %+v, want session/42/alice", out)
	}
}

func TestVerifyTypPinsType(t *testing.T) {
	s, _ := NewSigner([]string{secretA})
	tok, _ := s.Sign(Claims{Typ: TypState, Exp: time.Now().Add(time.Hour)})
	if _, err := s.VerifyTyp(tok, TypState); err != nil {
		t.Fatalf("matching type must verify: %v", err)
	}
	if _, err := s.VerifyTyp(tok, TypSession); err == nil {
		t.Fatal("mismatched type must be rejected (a state token must not pass as a session)")
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	s, _ := NewSigner([]string{secretA})
	tok, _ := s.Sign(Claims{Typ: TypSession, GitHubID: 1, Exp: time.Now().Add(-time.Second)})
	if _, err := s.verify(tok); err == nil {
		t.Fatal("expired token must be rejected")
	}
}

func TestVerifyRejectsTamper(t *testing.T) {
	s, _ := NewSigner([]string{secretA})
	tok, _ := s.Sign(Claims{Typ: TypSession, GitHubID: 1, Exp: time.Now().Add(time.Hour)})

	payload, sig, _ := strings.Cut(tok, ".")
	tampered := flipFirst(payload) + "." + sig
	if _, err := s.verify(tampered); err == nil {
		t.Fatal("tampered payload must be rejected")
	}

	for _, bad := range []string{"", ".", "noseparator", "a.", ".b", "a.b.c"} {
		if _, err := s.verify(bad); err == nil {
			t.Fatalf("malformed token %q must be rejected", bad)
		}
	}
}

// TestVerifyRotation verifies the rotation property: a Signer holding [new, old]
// accepts tokens minted under either secret.
func TestVerifyRotation(t *testing.T) {
	signNew, _ := NewSigner([]string{secretB})
	tokNew, _ := signNew.Sign(Claims{Typ: TypCLI, GitHubID: 7, Exp: time.Now().Add(time.Hour)})

	signOld, _ := NewSigner([]string{secretA})
	tokOld, _ := signOld.Sign(Claims{Typ: TypCLI, GitHubID: 8, Exp: time.Now().Add(time.Hour)})

	rotating, _ := NewSigner([]string{secretB, secretA})
	if c, err := rotating.verify(tokNew); err != nil || c.GitHubID != 7 {
		t.Fatalf("new-key token must verify under rotation: c=%+v err=%v", c, err)
	}
	if c, err := rotating.verify(tokOld); err != nil || c.GitHubID != 8 {
		t.Fatalf("old-key token must still verify under rotation: c=%+v err=%v", c, err)
	}

	// A signer that only knows the new key must NOT verify an old-key token.
	if _, err := signNew.verify(tokOld); err == nil {
		t.Fatal("old-key token must fail once the old secret is dropped")
	}
}

// flipFirst flips the first byte of s into a different base64url character so the
// decoded payload changes.
func flipFirst(s string) string {
	if s == "" {
		return "A"
	}
	b := []byte(s)
	if b[0] == 'A' {
		b[0] = 'B'
	} else {
		b[0] = 'A'
	}
	return string(b)
}
