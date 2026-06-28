package auth

import (
	"crypto/subtle"

	"golang.org/x/oauth2"
)

// VerifyPKCE constant-time-compares the RFC 7636 S256 challenge of verifier against
// the stored challenge; an empty verifier or challenge fails closed.
func VerifyPKCE(verifier, challenge string) bool {
	if verifier == "" || challenge == "" {
		return false
	}
	return subtle.ConstantTimeCompare(
		[]byte(oauth2.S256ChallengeFromVerifier(verifier)), []byte(challenge)) == 1
}
