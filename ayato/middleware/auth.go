package middleware

import (
	"sync/atomic"

	"github.com/Hayao0819/Kamisato/ayato/auth"
)

type Settings struct {
	CookieName             string
	PublicOrigin           string
	SelfOrigin             string
	AllowLegacySignerBasic bool
}

func (settings Settings) normalized() Settings {
	if settings.CookieName == "" {
		settings.CookieName = "__Host-ayato_session"
	}
	return settings
}

// adminChecker keeps HTTP authentication independent of the allowlist storage.
type adminChecker interface {
	IsAdmin(id int64) bool
}

// logTokenConsumer redeems a one-time SSE log token, returning its bound job ID.
type logTokenConsumer interface {
	ConsumeLogToken(token string) (jobID string, ok bool, err error)
}

type Middleware struct {
	settings  Settings
	checker   adminChecker
	signer    *auth.Signer
	ci        *auth.CIAuthorizer
	denylist  auth.RevocationChecker
	logTokens logTokenConsumer

	// Each RateLimit call site gets an independent counter namespace.
	limiter *RateLimiter
	rlScope atomic.Int64
}

func New(settings Settings) *Middleware {
	return &Middleware{settings: settings.normalized()}
}

func (m *Middleware) WithAuth(checker adminChecker, signer *auth.Signer) *Middleware {
	m.checker = checker
	m.signer = signer
	return m
}

// WithDenylist enables per-token and refresh-family revocation checks.
func (m *Middleware) WithDenylist(denylist auth.RevocationChecker) *Middleware {
	m.denylist = denylist
	return m
}

// WithLogTokens enables one-time browser access to SSE build logs.
func (m *Middleware) WithLogTokens(tokens logTokenConsumer) *Middleware {
	m.logTokens = tokens
	return m
}

func (m *Middleware) sessionCookieName() string {
	return m.settings.CookieName
}

func (m *Middleware) requestResolver() auth.RequestResolver {
	return auth.NewRequestResolver(m.signer, m.denylist, m.sessionCookieName())
}

// Gin context keys for the resolved requester (audit logging).
const (
	ctxGitHubID = "auth_github_id"
	ctxLogin    = "auth_login"
	ctxVia      = "auth_via"
)
