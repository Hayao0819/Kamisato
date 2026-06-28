package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	bearerTTL  = 7 * 24 * time.Hour
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

// externalBase returns ayato's OWN external (scheme, "scheme://host") used for
// the OAuth redirect_uri and the cookie Secure decision. The callback always
// lands on ayato, so this prefers SelfOrigin (ayato's own URL) and falls back to
// PublicOrigin — the two are identical in the same-origin/BFF deployment, and
// diverge only when the SPA is served cross-origin (bearer mode). X-Forwarded-*
// is deliberately NOT consulted: gin's SetTrustedProxies does not gate
// c.GetHeader, so it is spoofable by any direct peer. The request-host fallback
// runs only when neither origin is configured (OAuth disabled).
func (h *Handler) externalBase(c *gin.Context) (scheme, base string) {
	if h.cfg != nil {
		if s, b, ok := originOf(h.cfg.Auth.SelfOrigin); ok {
			return s, b
		}
		if s, b, ok := originOf(h.cfg.Auth.PublicOrigin); ok {
			return s, b
		}
	}
	s := "http"
	if c.Request.TLS != nil {
		s = "https"
	}
	return s, s + "://" + c.Request.Host
}

// spaOrigin returns the browser-facing SPA origin (PublicOrigin), used as the
// exact postMessage target for the web-bearer login. Empty when unset.
func (h *Handler) spaOrigin() string {
	if h.cfg != nil {
		if _, b, ok := originOf(h.cfg.Auth.PublicOrigin); ok {
			return b
		}
	}
	return ""
}

// originOf parses raw into its (scheme, "scheme://host") origin.
func originOf(raw string) (scheme, base string, ok bool) {
	if raw == "" {
		return "", "", false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", "", false
	}
	return u.Scheme, u.Scheme + "://" + u.Host, true
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

// webAuthBridgeTmpl is the popup page the web-bearer callback returns. It posts
// the one-time code (and the SPA's original state) to its opener at the exact
// SPA origin, then closes. The payload is base64-encoded JSON so no value is
// interpolated into HTML/JS, and the postMessage target is the configured SPA
// origin (never "*"), so the code reaches only the legitimate opener.
const webAuthBridgeTmpl = `<!doctype html><meta charset="utf-8"><title>Signing in…</title><body><script>
(function () {
  try {
    var d = JSON.parse(atob(%q));
    if (window.opener) {
      window.opener.postMessage({ type: "ayato-auth", code: d.code, state: d.state }, %q);
    }
  } catch (e) {}
  window.close();
})();
</script>Signing in…</body>`

// renderWebAuthBridge writes the popup bridge page that hands the one-time code
// back to the SPA. It requires PublicOrigin (the postMessage target); without it
// there is no trusted opener to deliver the code to, so it fails closed.
func (h *Handler) renderWebAuthBridge(c *gin.Context, code, state string) {
	origin := h.spaOrigin()
	if origin == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "public_origin not configured"})
		return
	}
	payload, err := json.Marshal(struct {
		Code  string `json:"code"`
		State string `json:"state"`
	}{Code: code, State: state})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encode"})
		return
	}
	b64 := base64.StdEncoding.EncodeToString(payload)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, fmt.Sprintf(webAuthBridgeTmpl, b64, origin))
}
