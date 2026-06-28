package handler

import (
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	githuboauth "golang.org/x/oauth2/github"
)

const (
	githubUserAPI    = "https://api.github.com/user"
	githubScope      = "read:user"
	oauthCallbackURI = "/api/unstable/auth/github/callback"
	// oauthStateCookieName carries the web-flow binding nonce that ties an OAuth
	// state to the browser that started it (login-CSRF / fixation defense).
	oauthStateCookieName = "ayato_oauth_state"
)

// Token lifetimes. Sessions and CLI tokens are revoked by de-allowlisting the id
// or rotating the signer secret; the short-lived code/state windows bound replay.
const (
	sessionTTL = 7 * 24 * time.Hour
	tokenTTL   = 90 * 24 * time.Hour
	codeTTL    = 60 * time.Second
	stateTTL   = 10 * time.Minute
)

// githubUser is the subset of GET /user we rely on.
type githubUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

// oauthEnabled reports whether the GitHub login flow is configured. Auth is
// "wired" when the signer is present (the service, which owns the allowlist, is
// always present); GitHub login additionally needs the client id/secret.
func (h *Handler) oauthEnabled() bool {
	return h.signer != nil && h.cfg != nil && h.cfg.Auth.GitHub.ClientID != "" && h.cfg.Auth.GitHub.ClientSecret != ""
}

// externalBase returns the external (scheme, "scheme://host") used for the OAuth
// redirect_uri and the cookie Secure decision. It is derived from PublicOrigin
// only: gin's SetTrustedProxies does NOT gate c.GetHeader, so X-Forwarded-* is
// spoofable by any direct peer and must not feed these values. The request-host
// fallback runs only when PublicOrigin is unset (OAuth disabled; conf.Validate
// makes PublicOrigin mandatory whenever GitHub login is enabled).
func (h *Handler) externalBase(c *gin.Context) (scheme, base string) {
	if h.cfg != nil && h.cfg.Auth.PublicOrigin != "" {
		if u, err := url.Parse(h.cfg.Auth.PublicOrigin); err == nil && u.Scheme != "" && u.Host != "" {
			return u.Scheme, u.Scheme + "://" + u.Host
		}
	}
	s := "http"
	if c.Request.TLS != nil {
		s = "https"
	}
	return s, s + "://" + c.Request.Host
}

// oauthConfig builds the confidential OAuth2 client with the redirect_uri
// pinned to this server's callback under the external origin.
func (h *Handler) oauthConfig(c *gin.Context) *oauth2.Config {
	_, base := h.externalBase(c)
	return &oauth2.Config{
		ClientID:     h.cfg.Auth.GitHub.ClientID,
		ClientSecret: h.cfg.Auth.GitHub.ClientSecret,
		Endpoint:     githuboauth.Endpoint,
		RedirectURL:  base + oauthCallbackURI,
		Scopes:       []string{githubScope},
	}
}

// setOAuthStateCookie stores the web-flow binding nonce in a host-only,
// HttpOnly, SameSite=Lax cookie. Lax is required so the cookie returns on the
// top-level cross-site redirect from GitHub back to the callback.
func (h *Handler) setOAuthStateCookie(c *gin.Context, nonce string, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    nonce,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearOAuthStateCookie(c *gin.Context, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// setSessionCookie sets the host-only session cookie. http.SetCookie is used
// directly (instead of gin's c.SetCookie) so we can omit Domain entirely,
// yielding a host-only cookie.
func (h *Handler) setSessionCookie(c *gin.Context, value string, secure bool, maxAge int) {
	ck := &http.Cookie{
		Name:     h.cfg.Auth.CookieName(),
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		// Domain intentionally empty -> host-only cookie.
	}
	http.SetCookie(c.Writer, ck)
}
