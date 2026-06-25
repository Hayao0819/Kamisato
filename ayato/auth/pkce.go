package auth

import (
	"crypto/subtle"

	"golang.org/x/oauth2"
)

// VerifyPKCE reports whether the RFC 7636 S256 challenge of verifier equals the
// stored challenge, constant-time. An empty verifier or challenge fails closed.
// The S256 transform is delegated to x/oauth2 rather than hand-rolled.
func VerifyPKCE(verifier, challenge string) bool {
	if verifier == "" || challenge == "" {
		return false
	}
	return subtle.ConstantTimeCompare(
		[]byte(oauth2.S256ChallengeFromVerifier(verifier)), []byte(challenge)) == 1
}
