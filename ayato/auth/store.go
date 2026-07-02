// Package auth implements ayato's GitHub-OAuth auth. Every auth artifact (web
// sessions, CLI tokens, one-time codes, pending OAuth states) is a stateless
// HMAC-signed envelope carried by the client, so this package mints/verifies but
// never touches storage. Fail-closed throughout; the admin allowlist is the only
// server-side state and lives in the repository layer, not here.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/Hayao0819/Kamisato/internal/errwrap"
)

func randToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", errwrap.WrapErr(err, "auth: read random")
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// NewState returns a fresh crypto-random nonce: the web OAuth binding nonce (only
// its hash is signed into the state token) and the CLI loopback fallback state.
func NewState() (string, error) { return randToken(32) }

// NewJTI returns a fresh 128-bit token id that makes a minted token individually
// revocable via the denylist.
func NewJTI() (string, error) { return randToken(16) }

// HashHex returns the hex SHA-256 of s, used to bind the OAuth state token to a
// browser cookie nonce without carrying the plaintext nonce.
func HashHex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum[:])
}
