package aur

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/internal/kayoproto"
)

// KeyID is a human-comparable handle (first 16 hex of SHA-256) for logs and
// out-of-band pin confirmation, never for crypto decisions.
func KeyID(pub ed25519.PublicKey) string {
	return kayoproto.KeyID(pub)
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
		return nil, errwrap.WrapErr(err, "aur: decode signing seed")
	}
	if len(seed) != ed25519.SeedSize { // 32; NewKeyFromSeed panics otherwise
		return nil, errwrap.NewErrf("aur: signing seed must be %d bytes, got %d", ed25519.SeedSize, len(seed))
	}
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	return &CatalogSigner{keyID: KeyID(pub), priv: priv, pub: pub, ttl: ttl}, nil
}

// NewCatalogSignerFromEnv builds the signer from AYATO_AUR_SIGNING_SEED, mirroring
// how the repository factory loads the DB signing key: the seed is a private key,
// so it comes only from the environment, never a config file. An unset seed is not
// an error — it returns a nil signer so the catalog is served unsigned (legacy) —
// but a present-yet-malformed seed fails closed.
func NewCatalogSignerFromEnv(ttl time.Duration) (*CatalogSigner, error) {
	seed := os.Getenv("AYATO_AUR_SIGNING_SEED")
	if seed == "" {
		return nil, nil
	}
	return NewCatalogSigner(seed, ttl)
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
		return kayoproto.CatalogEnvelope{}, errwrap.WrapErr(err, "aur: marshal signed payload")
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
		return "", errwrap.WrapErr(err, "aur: generate key")
	}
	return base64.StdEncoding.EncodeToString(priv.Seed()), nil
}
