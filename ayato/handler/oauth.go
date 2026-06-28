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

// GitHub login needs the signer wired plus a configured client id/secret.
func (h *Handler) oauthEnabled() bool {
	return h.signer != nil && h.cfg != nil && h.cfg.Auth.GitHub.ClientID != "" && h.cfg.Auth.GitHub.ClientSecret != ""
}

// ayato's own origin for the OAuth redirect_uri and the cookie Secure flag.
// Prefers SelfOrigin, then PublicOrigin. X-Forwarded-* is ignored because gin
// does not gate c.GetHeader, so it is spoofable; the request host is used only
// when neither origin is configured.
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

// SPA origin (PublicOrigin) used as the exact postMessage target; empty when unset.
func (h *Handler) spaOrigin() string {
	if h.cfg != nil {
		if _, b, ok := originOf(h.cfg.Auth.PublicOrigin); ok {
			return b
		}
	}
	return ""
}

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

// SameSite=Lax so the cookie returns on GitHub's top-level cross-site redirect back.
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

// Uses http.SetCookie (not gin's c.SetCookie) to omit Domain for a host-only cookie.
func (h *Handler) setSessionCookie(c *gin.Context, value string, secure bool, maxAge int) {
	ck := &http.Cookie{
		Name:     h.cfg.Auth.CookieName(),
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(c.Writer, ck)
}

// Popup that postMessages the one-time code to its opener at the exact SPA
// origin (never "*"), then closes. The payload is base64 JSON so nothing is
// interpolated into HTML/JS.
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

// Fails closed without PublicOrigin: there is no trusted postMessage target to
// deliver the code to.
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
