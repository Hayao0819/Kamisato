// Package auth implements ayato's identity and authorization. Interactive users
// authenticate via GitHub OAuth: every user artifact (web sessions, CLI tokens,
// one-time codes, pending OAuth states) is a stateless HMAC-signed envelope carried
// by the client, so the Signer mints/verifies but never touches storage.
// Non-interactive CI uploads are authorized separately by CIAuthorizer, which
// verifies a repository identity (API key or GitHub OIDC) rather than a user.
// Fail-closed throughout; the admin allowlist is the only server-side state and
// lives in the repository layer, not here.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

// NewOpaqueToken returns a URL-safe token with byteLength bytes of
// cryptographically secure entropy.
func NewOpaqueToken(byteLength int) (string, error) {
	if byteLength <= 0 {
		return "", errors.NewErr("auth: token length must be positive")
	}
	b, err := randomBytes(byteLength)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// NewState returns a fresh crypto-random nonce: the web OAuth binding nonce (only
// its hash is signed into the state token) and the CLI loopback fallback state.
func NewState() (string, error) { return NewOpaqueToken(32) }

// NewJTI returns a fresh 128-bit token id that makes a minted token individually
// revocable via the denylist.
func NewJTI() (string, error) { return NewOpaqueToken(16) }

// NewDeviceCode returns the opaque high-entropy device_code the polling client
// presents at the device token endpoint (RFC 8628).
func NewDeviceCode() (string, error) { return NewOpaqueToken(32) }

// userCodeAlphabet is the RFC 8628 §6.1 recommended user-code charset: 20
// unambiguous uppercase consonants (no vowels, so no accidental words, and none of
// 0/O/1/I/L) so a human can read and type it back without confusion.
const userCodeAlphabet = "BCDFGHJKLMNPQRSTVWXZ"

// NewUserCode returns the short, human-typable user_code as two groups of four
// (e.g. "BCDF-GHJK").
func NewUserCode() (string, error) {
	const groups, per = 2, 4
	b, err := randomBytes(groups * per)
	if err != nil {
		return "", err
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

func randomBytes(length int) ([]byte, error) {
	value := make([]byte, length)
	if _, err := rand.Read(value); err != nil {
		return nil, errors.WrapErr(err, "auth: read random")
	}
	return value, nil
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
