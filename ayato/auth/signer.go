package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// Token type discriminators. Verify checks the envelope HMAC and expiry; callers
// must additionally pin the expected Typ via VerifyTyp so a token minted for one
// purpose (e.g. an OAuth state) can never be replayed as another (e.g. a
// session).
const (
	TypSession = "session"
	TypCLI     = "cli"
	TypCode    = "code"
	TypState   = "state"
)

// minSecretBytes is the minimum HMAC key length. 32 bytes (256 bits) matches the
// SHA-256 output and keeps the key from being the weak point.
const minSecretBytes = 32

// Claims is the stateless token payload. The same struct backs every token type;
// Typ disambiguates them and unused fields are omitted from the wire form. Exp
// is always set — Verify rejects an elapsed token regardless of type.
type Claims struct {
	Typ       string
	GitHubID  int64
	Login     string
	Name      string `json:",omitempty"`
	Port      int    `json:",omitempty"`
	Challenge string `json:",omitempty"`
	Binding   string `json:",omitempty"`
	CLIState  string `json:",omitempty"`
	CLI       bool   `json:",omitempty"`
	Exp       time.Time
}

// Signer mints and verifies stateless HMAC-SHA256 tokens. secrets[0] signs; ALL
// secrets are tried on verify, so a secret can be rotated by prepending a new
// one while old tokens keep verifying against the trailing entries.
type Signer struct {
	secrets [][]byte
}

// NewSigner builds a Signer from one or more shared secrets. The first secret
// signs; every secret verifies. Each secret must be at least 32 bytes; an empty
// list or any short secret is rejected (fail-closed configuration).
func NewSigner(secrets []string) (*Signer, error) {
	if len(secrets) == 0 {
		return nil, utils.NewErr("auth: at least one session secret is required")
	}
	keys := make([][]byte, 0, len(secrets))
	for _, s := range secrets {
		if len(s) < minSecretBytes {
			return nil, utils.NewErrf("auth: session secret must be at least %d bytes", minSecretBytes)
		}
		keys = append(keys, []byte(s))
	}
	return &Signer{secrets: keys}, nil
}

// mac returns the HMAC-SHA256 of payloadB64 under key.
func mac(key []byte, payloadB64 string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(payloadB64))
	return h.Sum(nil)
}

// Sign serializes c and returns base64url(payload) + "." + base64url(HMAC). The
// signing key is secrets[0].
func (s *Signer) Sign(c Claims) (string, error) {
	payload, err := json.Marshal(c)
	if err != nil {
		return "", utils.WrapErr(err, "auth: marshal claims")
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	sigB64 := base64.RawURLEncoding.EncodeToString(mac(s.secrets[0], payloadB64))
	return payloadB64 + "." + sigB64, nil
}

// Verify checks the envelope signature against every configured secret
// (constant-time), unmarshals the payload, and rejects an expired token. It does
// NOT check Typ — callers use VerifyTyp to pin the expected type.
func (s *Signer) Verify(token string) (*Claims, error) {
	payloadB64, sigB64, ok := strings.Cut(token, ".")
	if !ok || payloadB64 == "" || sigB64 == "" {
		return nil, utils.NewErr("auth: malformed token")
	}
	want, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return nil, utils.NewErr("auth: malformed token signature")
	}
	matched := false
	for _, key := range s.secrets {
		if subtle.ConstantTimeCompare(mac(key, payloadB64), want) == 1 {
			matched = true
			break
		}
	}
	if !matched {
		return nil, utils.NewErr("auth: bad token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, utils.NewErr("auth: malformed token payload")
	}
	var c Claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return nil, utils.NewErr("auth: malformed token claims")
	}
	if time.Now().After(c.Exp) {
		return nil, utils.NewErr("auth: token expired")
	}
	return &c, nil
}

// VerifyTyp verifies token and additionally requires its Typ to equal typ. A
// token minted for a different purpose is rejected even with a valid signature.
func (s *Signer) VerifyTyp(token, typ string) (*Claims, error) {
	c, err := s.Verify(token)
	if err != nil {
		return nil, err
	}
	if c.Typ != typ {
		return nil, utils.NewErrf("auth: token type %q, want %q", c.Typ, typ)
	}
	return c, nil
}
