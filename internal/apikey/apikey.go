// Package apikey verifies shared-secret API keys for service-to-service calls
// (ayato -> miko), using a constant-time comparison.
package apikey

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Verifier checks presented keys against a configured set. Multiple keys are
// allowed so a key can be rotated by accepting both old and new for a window.
type Verifier struct {
	keys [][]byte
}

func NewVerifier(keys []string) *Verifier {
	v := &Verifier{}
	for _, k := range keys {
		if k != "" {
			v.keys = append(v.keys, []byte(k))
		}
	}
	return v
}

// Enabled reports whether any key is configured.
func (v *Verifier) Enabled() bool { return len(v.keys) > 0 }

// Valid reports whether presented matches a configured key. It compares against
// every key (no early return) to avoid leaking which key matched via timing.
func (v *Verifier) Valid(presented string) bool {
	p := []byte(presented)
	matched := 0
	for _, k := range v.keys {
		matched |= subtle.ConstantTimeCompare(p, k)
	}
	return matched == 1
}

// Middleware requires a valid key when keys are configured. With none
// configured it passes through (closed-network trust only).
func (v *Verifier) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if v.Enabled() && !v.Valid(FromRequest(c)) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	}
}

// FromRequest extracts the key from "Authorization: Bearer <key>" or the
// "X-API-Key" header.
func FromRequest(c *gin.Context) string {
	if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return c.GetHeader("X-API-Key")
}

// Header is the canonical header ayato uses to send the key to miko.
const Header = "X-API-Key"
