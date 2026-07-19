// Package apikey verifies named service credentials.
package apikey

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	Header = "X-API-Key"

	ScopeAll         = "*"
	ScopeBuildSubmit = "build:submit"
	ScopeBuildRead   = "build:read"
	ScopeBuildCancel = "build:cancel"
	ScopeBuildAdmin  = "build:admin"
	ScopeSign        = "sign"
)

type Entry struct {
	// Name identifies a key; Principal identifies its owner.
	Name      string
	Principal string
	Key       string
	Scopes    []string
}

type Principal struct {
	// Name identifies the owner and KeyID identifies the credential.
	Name   string
	KeyID  string
	scopes map[string]bool
}

func (p *Principal) Allows(scope string) bool {
	if p == nil {
		return false
	}
	if scope == "" || p.scopes[ScopeAll] || p.scopes[scope] {
		return true
	}
	return strings.HasPrefix(scope, "build:") && p.scopes[ScopeBuildAdmin]
}

type verifierEntry struct {
	keyID     string
	principal string
	key       []byte
	scopes    map[string]bool
}

type Verifier struct {
	entries []verifierEntry
}

// NewVerifier accepts legacy key configurations.
func NewVerifier(keys []string) *Verifier {
	entries := make([]Entry, 0, len(keys))
	for index, key := range keys {
		entries = append(entries, Entry{
			Name:   "legacy-" + strconv.Itoa(index+1),
			Key:    key,
			Scopes: []string{ScopeAll},
		})
	}
	return NewScopedVerifier(entries)
}

func NewScopedVerifier(entries []Entry) *Verifier {
	verifier := &Verifier{}
	for _, entry := range entries {
		if entry.Name == "" || entry.Key == "" {
			continue
		}
		scopes := make(map[string]bool, len(entry.Scopes))
		for _, scope := range entry.Scopes {
			if scope != "" {
				scopes[scope] = true
			}
		}
		principal := entry.Principal
		if principal == "" {
			principal = entry.Name
		}
		verifier.entries = append(verifier.entries, verifierEntry{
			keyID:     entry.Name,
			principal: principal,
			key:       []byte(entry.Key),
			scopes:    scopes,
		})
	}
	return verifier
}

func (v *Verifier) Enabled() bool { return v != nil && len(v.entries) > 0 }

func (v *Verifier) Authenticate(presented string) (*Principal, bool) {
	if v == nil {
		return nil, false
	}
	key := []byte(presented)
	matched := -1
	for index := range v.entries {
		if subtle.ConstantTimeCompare(key, v.entries[index].key) == 1 {
			matched = index
		}
	}
	if matched < 0 {
		return nil, false
	}
	entry := v.entries[matched]
	return &Principal{Name: entry.principal, KeyID: entry.keyID, scopes: entry.scopes}, true
}

func (v *Verifier) Valid(presented string) bool {
	_, ok := v.Authenticate(presented)
	return ok
}

const principalContextKey = "service_api_key_principal"

// Middleware authenticates a key and checks the requested scopes.
func (v *Verifier) Middleware(scopes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !v.Enabled() {
			c.Next()
			return
		}
		principal, ok := v.Authenticate(FromRequest(c))
		if !ok {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Set(principalContextKey, principal)
		for _, scope := range scopes {
			if !principal.Allows(scope) {
				slog.Warn("service API key scope denied", "principal", principal.Name, "key_id", principal.KeyID, "scope", scope, "method", c.Request.Method, "path", c.FullPath())
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}
		if strings.HasPrefix(principal.KeyID, "legacy-") {
			slog.Warn("legacy full-scope Miko API key used", "key_id", principal.KeyID, "method", c.Request.Method, "path", c.FullPath())
		}
		c.Next()
	}
}

func PrincipalFrom(c *gin.Context) (*Principal, bool) {
	value, ok := c.Get(principalContextKey)
	if !ok {
		return nil, false
	}
	principal, ok := value.(*Principal)
	return principal, ok
}

func FromRequest(c *gin.Context) string { return c.GetHeader(Header) }
