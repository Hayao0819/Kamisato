// Package saraproto defines the private sara<->ayato exchange types. It is
// internal on purpose: unlike pkg/aurweb (a reusable aurweb-compatible library),
// this protocol is specific to Kamisato and free to carry trust and attestation
// fields the public aurweb RPC cannot.
package saraproto

import (
	"encoding/json"
	"time"

	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

// Catalog is an ayato instance's managed packages and the git URL each pkgbase
// is cloned from.
type Catalog struct {
	Packages []aurweb.Pkg      `json:"packages"`
	Sources  map[string]string `json:"sources"` // pkgbase -> git clone URL
}

// SignedPayload is the EXACT byte sequence ayato signs and sara verifies. It is
// embedded verbatim (a json.RawMessage) in CatalogEnvelope, so both sides operate
// on identical bytes with no JSON re-canonicalization. Every field is
// authenticated, so the freshness timestamps cannot be moved without breaking the
// signature.
type SignedPayload struct {
	KeyID     string    `json:"key_id"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at,omitzero"`
	Catalog   Catalog   `json:"catalog"`
}

// CatalogEnvelope is the wire form at the catalog endpoint. The detached ed25519
// signature covers Payload's raw bytes verbatim, carried in the body (not a
// header) so a header-normalizing proxy can't strip it. An empty Payload means a
// legacy unsigned ayato (bare Catalog was served); Alg=="none" means signing is
// disabled. sara refuses an unsigned/legacy catalog for any pinned source.
type CatalogEnvelope struct {
	Payload   json.RawMessage `json:"payload,omitempty"`   // signed SignedPayload bytes, verbatim
	Alg       string          `json:"alg,omitempty"`       // "ed25519" | "none"
	KeyID     string          `json:"key_id,omitempty"`    // rotation hint (unauthenticated)
	Signature string          `json:"signature,omitempty"` // base64-std ed25519 sig over Payload
}
