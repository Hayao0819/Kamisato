package ayatosrc

import (
	"crypto/ed25519"
	"encoding/base64"
	"time"

	"github.com/Hayao0819/Kamisato/internal/kayoproto"
	"github.com/Hayao0819/Kamisato/internal/utils"
)

// Verifier checks an ayato catalog's detached Ed25519 signature and freshness.
type Verifier struct {
	pub    ed25519.PublicKey
	keyID  string
	maxAge time.Duration
	leeway time.Duration
}

// NewVerifier builds a verifier from a base64 32-byte public key. maxAge is the
// staleness ceiling kayo enforces independently of the catalog's own ExpiresAt;
// it must be non-zero so a pinned source can never be frozen at a stale snapshot.
func NewVerifier(pubB64 string, maxAge time.Duration) (*Verifier, error) {
	pub, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil {
		return nil, utils.WrapErr(err, "ayato: decode catalog public key")
	}
	if len(pub) != ed25519.PublicKeySize { // 32; ed25519.Verify panics on a wrong size
		return nil, utils.NewErrf("ayato: public key must be %d bytes, got %d", ed25519.PublicKeySize, len(pub))
	}
	// The freshness logic treats maxAge<=0 as "no ceiling", which would let a
	// verified catalog never age out (the freeze attack); enforce the contract here.
	if maxAge <= 0 {
		return nil, utils.NewErr("ayato: maxAge must be positive")
	}
	return &Verifier{pub: ed25519.PublicKey(pub), keyID: kayoproto.KeyID(pub), maxAge: maxAge, leeway: time.Minute}, nil
}

func (v *Verifier) KeyID() string { return v.keyID }

// VerifyPayload checks the detached signature over the EXACT payload bytes. Call
// it BEFORE unmarshaling the payload.
func (v *Verifier) VerifyPayload(payload []byte, sigB64 string) error {
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return utils.WrapErr(err, "ayato: decode catalog signature")
	}
	if len(sig) != ed25519.SignatureSize {
		return utils.NewErrf("ayato: signature must be %d bytes, got %d", ed25519.SignatureSize, len(sig))
	}
	if !ed25519.Verify(v.pub, payload, sig) {
		return utils.NewErr("ayato: catalog signature does not verify")
	}
	return nil
}

// CheckFreshness uses the SIGNED timestamps and the persisted watermark to stop
// replay, rollback, and freeze. lastIssued is the highest IssuedAt kayo has
// accepted for this source (0 on first contact).
func (v *Verifier) CheckFreshness(issuedAt, expiresAt, lastIssued time.Time) error {
	now := time.Now()
	if issuedAt.IsZero() {
		return utils.NewErr("ayato: catalog missing issued_at")
	}
	if issuedAt.After(now.Add(v.leeway)) {
		return utils.NewErrf("ayato: catalog issued in the future (%s)", issuedAt)
	}
	if !expiresAt.IsZero() && now.After(expiresAt.Add(v.leeway)) {
		return utils.NewErrf("ayato: catalog expired at %s", expiresAt)
	}
	if v.maxAge > 0 && now.Sub(issuedAt) > v.maxAge+v.leeway {
		return utils.NewErrf("ayato: catalog too old (issued %s, max age %s)", issuedAt, v.maxAge)
	}
	if !lastIssued.IsZero() && !issuedAt.After(lastIssued) {
		return utils.NewErrf("ayato: catalog rollback (issued %s <= last accepted %s)", issuedAt, lastIssued)
	}
	return nil
}
