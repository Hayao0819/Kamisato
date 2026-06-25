package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestPKCEMatchAndMismatch(t *testing.T) {
	verifier := "test-verifier-0123456789-abcdefghijklmnop"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	if !VerifyPKCE(verifier, challenge) {
		t.Fatalf("matching verifier/challenge must pass")
	}
	if VerifyPKCE("wrong-verifier", challenge) {
		t.Fatalf("mismatched verifier must fail")
	}
	if VerifyPKCE("", challenge) || VerifyPKCE(verifier, "") {
		t.Fatalf("empty verifier/challenge must fail closed")
	}
}
