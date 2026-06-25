// Package auth implements ayato's GitHub-OAuth authentication subsystem. All
// auth state is stateless-signed (web sessions, CLI tokens, one-time CLI codes,
// and pending OAuth states are HMAC-signed envelopes carried by the client)
// EXCEPT the admin allowlist, which is the single piece of server-side state and
// lives on the shared kv.Store. Everything is fail-closed: an empty allowlist
// denies, unknown ids are rejected, and signature checks are constant-time.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// randToken returns n random bytes encoded as URL-safe base64 without padding.
func randToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", utils.WrapErr(err, "auth: read random")
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// NewState returns a fresh crypto-random nonce. It is used to mint the web
// OAuth binding nonce (only its hash is signed into the state token) and as the
// CLI fallback state echoed back on the loopback redirect.
func NewState() (string, error) { return randToken(32) }

// HashHex returns the hex SHA-256 of s. Used to bind the web OAuth state token
// to a browser-held cookie nonce without carrying the plaintext nonce.
func HashHex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum[:])
}
