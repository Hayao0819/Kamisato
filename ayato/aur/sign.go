package aur

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/Hayao0819/Kamisato/internal/kayoproto"
	"github.com/Hayao0819/Kamisato/internal/utils"
)

// KeyID is a human-comparable handle (first 16 hex of SHA-256) for logs and
// out-of-band pin confirmation, never for crypto decisions.
func KeyID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return hex.EncodeToString(sum[:])[:16]
}

// CatalogSigner is ayato's Ed25519 identity for the catalog: ayato alone holds the
// private seed and kayo verifies with a pinned public key — the asymmetric anchor
// the unsigned AUR cannot provide.
type CatalogSigner struct {
	keyID string
	priv  ed25519.PrivateKey
	pub   ed25519.PublicKey
	ttl   time.Duration
}

// NewCatalogSigner builds a signer from a base64 32-byte seed. The seed must come
// from the environment (AYATO_AUR_SIGNING_SEED), never a config file on disk.
func NewCatalogSigner(seedB64 string, ttl time.Duration) (*CatalogSigner, error) {
	seed, err := base64.StdEncoding.DecodeString(seedB64)
	if err != nil {
		return nil, utils.WrapErr(err, "aur: decode signing seed")
	}
	if len(seed) != ed25519.SeedSize { // 32; NewKeyFromSeed panics otherwise
		return nil, utils.NewErrf("aur: signing seed must be %d bytes, got %d", ed25519.SeedSize, len(seed))
	}
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	return &CatalogSigner{keyID: KeyID(pub), priv: priv, pub: pub, ttl: ttl}, nil
}

func (s *CatalogSigner) KeyID() string        { return s.keyID }
func (s *CatalogSigner) PublicKeyB64() string { return base64.StdEncoding.EncodeToString(s.pub) }

// Sign returns a detached signature over the exact marshaled payload bytes (pure
// Ed25519, no pre-hash).
func (s *CatalogSigner) Sign(cat kayoproto.Catalog) (kayoproto.CatalogEnvelope, error) {
	now := time.Now().UTC()
	p := kayoproto.SignedPayload{KeyID: s.keyID, IssuedAt: now, Catalog: cat}
	if s.ttl > 0 {
		p.ExpiresAt = now.Add(s.ttl)
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return kayoproto.CatalogEnvelope{}, utils.WrapErr(err, "aur: marshal signed payload")
	}
	sig := ed25519.Sign(s.priv, payload)
	return kayoproto.CatalogEnvelope{
		Payload:   payload,
		Alg:       "ed25519",
		KeyID:     s.keyID,
		Signature: base64.StdEncoding.EncodeToString(sig),
	}, nil
}

// GenerateSeed returns a fresh base64-encoded Ed25519 seed for `ayato aur keygen`.
func GenerateSeed() (string, error) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return "", utils.WrapErr(err, "aur: generate key")
	}
	return base64.StdEncoding.EncodeToString(priv.Seed()), nil
}
