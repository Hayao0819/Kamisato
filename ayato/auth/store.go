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

// NewDeviceCode returns the opaque high-entropy device_code the polling client
// presents at the device token endpoint (RFC 8628).
func NewDeviceCode() (string, error) { return randToken(32) }

// userCodeAlphabet is the RFC 8628 §6.1 recommended user-code charset: 20
// unambiguous uppercase consonants (no vowels, so no accidental words, and none of
// 0/O/1/I/L) so a human can read and type it back without confusion.
const userCodeAlphabet = "BCDFGHJKLMNPQRSTVWXZ"

// NewUserCode returns the short, human-typable user_code as two groups of four
// (e.g. "BCDF-GHJK"). The user reads it off the CLI and types it into the
// verification page in any browser.
func NewUserCode() (string, error) {
	const groups, per = 2, 4
	b := make([]byte, groups*per)
	if _, err := rand.Read(b); err != nil {
		return "", errwrap.WrapErr(err, "auth: read random")
	}
	out := make([]byte, 0, groups*per+groups-1)
	for i, v := range b {
		if i > 0 && i%per == 0 {
			out = append(out, '-')
		}
		out = append(out, userCodeAlphabet[int(v)%len(userCodeAlphabet)])
	}
	return string(out), nil
}

// Device authorization outcomes (RFC 8628 polling states) shared by the device
// repository and the handlers so the two never drift on the wire strings.
const (
	DevicePending  = "pending"
	DeviceApproved = "approved"
	DeviceDenied   = "denied"
)

// HashHex returns the hex SHA-256 of s, used to bind the OAuth state token to a
// browser cookie nonce without carrying the plaintext nonce.
func HashHex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum[:])
}
