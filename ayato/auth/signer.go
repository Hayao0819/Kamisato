package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errwrap"
)

// Token type discriminators. Callers must pin the expected Typ via VerifyTyp so a
// token minted for one purpose can never be replayed as another.
const (
	TypSession = "session"
	TypCLI     = "cli"
	// TypCodeCLI/TypCodeWeb are the one-time PKCE codes for the CLI loopback and
	// the web-bearer (SPA) flows. They are distinct so a code minted for one flow
	// can never be redeemed at the other exchange endpoint.
	TypCodeCLI = "code_cli"
	TypCodeWeb = "code_web"
	TypState   = "state"
	// TypBearer is the web SPA session token, distinct from TypCLI so the two
	// delivery paths can carry different lifetimes.
	TypBearer = "bearer"
)

// minSecretBytes (32) matches the SHA-256 output so the key isn't the weak point.
const minSecretBytes = 32

// Claims is the stateless token payload shared by every token type; Typ
// disambiguates them. Exp is always set and verify rejects an elapsed token.
type Claims struct {
	Typ       string
	GitHubID  int64
	Login     string
	Name      string `json:",omitempty"`
	Port      int    `json:",omitempty"`
	Challenge string `json:",omitempty"`
	Binding   string `json:",omitempty"`
	CLIState  string `json:",omitempty"`
	// UserCode carries the RFC 8628 device user_code through the OAuth state so the
	// callback knows which pending device authorization to approve or deny.
	UserCode string `json:",omitempty"`
	CLI      bool   `json:",omitempty"`
	Web      bool   `json:",omitempty"`
	// Device marks the state token as belonging to the device-authorization flow.
	Device bool `json:",omitempty"`
	// JTI is a unique token id set on CLI tokens so a single token can be revoked
	// via the denylist. omitempty keeps tokens that never set it byte-identical.
	JTI string `json:",omitempty"`
	Exp time.Time
}

// Signer mints and verifies stateless HMAC-SHA256 tokens. secrets[0] signs; all
// secrets verify, so prepending a new secret rotates it while old tokens still verify.
type Signer struct {
	secrets [][]byte
}

// NewSigner builds a Signer from one or more shared secrets. Each must be at least
// 32 bytes; an empty list or any short secret is rejected (fail-closed).
func NewSigner(secrets []string) (*Signer, error) {
	if len(secrets) == 0 {
		return nil, errwrap.NewErr("auth: at least one session secret is required")
	}
	keys := make([][]byte, 0, len(secrets))
	for _, s := range secrets {
		if len(s) < minSecretBytes {
			return nil, errwrap.NewErrf("auth: session secret must be at least %d bytes", minSecretBytes)
		}
		keys = append(keys, []byte(s))
	}
	return &Signer{secrets: keys}, nil
}

func mac(key []byte, payloadB64 string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(payloadB64))
	return h.Sum(nil)
}

// Sign returns base64url(payload).base64url(HMAC), signed with secrets[0].
func (s *Signer) Sign(c Claims) (string, error) {
	payload, err := json.Marshal(c)
	if err != nil {
		return "", errwrap.WrapErr(err, "auth: marshal claims")
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	sigB64 := base64.RawURLEncoding.EncodeToString(mac(s.secrets[0], payloadB64))
	return payloadB64 + "." + sigB64, nil
}

// verify checks the signature against every secret (constant-time) and rejects an
// expired token. It does NOT check Typ — callers use VerifyTyp for that.
func (s *Signer) verify(token string) (*Claims, error) {
	payloadB64, sigB64, ok := strings.Cut(token, ".")
	if !ok || payloadB64 == "" || sigB64 == "" {
		return nil, errwrap.NewErr("auth: malformed token")
	}
	want, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return nil, errwrap.NewErr("auth: malformed token signature")
	}
	matched := false
	for _, key := range s.secrets {
		if subtle.ConstantTimeCompare(mac(key, payloadB64), want) == 1 {
			matched = true
			break
		}
	}
	if !matched {
		return nil, errwrap.NewErr("auth: bad token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, errwrap.NewErr("auth: malformed token payload")
	}
	var c Claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return nil, errwrap.NewErr("auth: malformed token claims")
	}
	if time.Now().After(c.Exp) {
		return nil, errwrap.NewErr("auth: token expired")
	}
	return &c, nil
}

// VerifyTyp verifies token and additionally requires Typ == typ, rejecting a token
// minted for another purpose even with a valid signature.
func (s *Signer) VerifyTyp(token, typ string) (*Claims, error) {
	c, err := s.verify(token)
	if err != nil {
		return nil, err
	}
	if c.Typ != typ {
		return nil, errwrap.NewErrf("auth: token type %q, want %q", c.Typ, typ)
	}
	return c, nil
}
