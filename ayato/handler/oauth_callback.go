package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/gin-gonic/gin"
)

// Completes both the web and CLI OAuth flows.
func (h *Handler) GitHubCallbackHandler(c *gin.Context) {
	if !h.oauthEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "github login not configured"})
		return
	}
	st, err := h.signer.VerifyTyp(c.Query("state"), auth.TypState)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid state"})
		return
	}

	// Web cookie flow: the state must be redeemed by the same browser (binding
	// cookie, cleared either way). CLI and web-bearer flows bind via PKCE instead.
	if !st.CLI && !st.Web {
		scheme, _ := h.externalBase(c)
		nonce, cerr := c.Cookie(oauthStateCookieName)
		h.clearOAuthStateCookie(c, scheme == "https")
		if cerr != nil || st.Binding == "" ||
			subtle.ConstantTimeCompare([]byte(auth.HashHex(nonce)), []byte(st.Binding)) != 1 {
			c.JSON(http.StatusForbidden, gin.H{"error": "state binding mismatch"})
			return
		}
	}

	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing code"})
		return
	}

	user, ok := h.resolveGitHubUser(c, code)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "github identity check failed"})
		return
	}

	// Fail-closed allowlist by numeric id.
	if !h.s.IsAdmin(user.ID) {
		slog.Warn("github login denied (not allowlisted)", "github_id", user.ID, "login", user.Login)
		c.JSON(http.StatusForbidden, gin.H{"error": "not allowed"})
		return
	}

	if st.CLI {
		h.finishCLILogin(c, st, user)
		return
	}
	if st.Web {
		h.finishWebBearerLogin(c, st, user)
		return
	}
	h.finishWebLogin(c, user)
}

// Any error fails closed (ok=false).
func (h *Handler) resolveGitHubUser(c *gin.Context, code string) (githubUser, bool) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	cfg := h.oauthConfig(c)
	tok, err := cfg.Exchange(ctx, code)
	if err != nil {
		slog.Warn("github code exchange failed", "error", err)
		return githubUser{}, false
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubUserAPI, nil)
	if err != nil {
		return githubUser{}, false
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	// The access token is used only for this /user call and never stored.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Warn("github /user request failed", "error", err)
		return githubUser{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Warn("github /user non-200", "status", resp.StatusCode)
		return githubUser{}, false
	}
	var u githubUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil || u.ID == 0 {
		return githubUser{}, false
	}
	return u, true
}

// Fresh session token every login (fixation defense); redirects to "/" only,
// never an attacker-supplied target.
func (h *Handler) finishWebLogin(c *gin.Context, user githubUser) {
	value, err := h.signer.Sign(auth.Claims{
		Typ:      auth.TypSession,
		GitHubID: user.ID,
		Login:    user.Login,
		Exp:      time.Now().Add(sessionTTL),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session"})
		return
	}
	scheme, _ := h.externalBase(c)
	h.setSessionCookie(c, value, scheme == "https", int(sessionTTL/time.Second))
	c.Redirect(http.StatusFound, "/")
}

// Returns a one-time CODE (never a token) to the server-reconstructed loopback,
// echoing ayaka's original state so its state-equality check passes.
func (h *Handler) finishCLILogin(c *gin.Context, st *auth.Claims, user githubUser) {
	oneTime, err := h.signer.Sign(auth.Claims{
		Typ:       auth.TypCode,
		GitHubID:  user.ID,
		Login:     user.Login,
		Challenge: st.Challenge,
		Exp:       time.Now().Add(codeTTL),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "code"})
		return
	}
	// Build the loopback from the integer port only, never a caller-supplied URL.
	target := url.URL{
		Scheme:   "http",
		Host:     "127.0.0.1:" + strconv.Itoa(st.Port),
		Path:     "/",
		RawQuery: url.Values{"code": {oneTime}, "state": {st.CLIState}}.Encode(),
	}
	c.Redirect(http.StatusFound, target.String())
}

// Returns a one-time CODE (never a token) to the SPA via the postMessage bridge;
// redeemed at /auth/web/exchange with PKCE.
func (h *Handler) finishWebBearerLogin(c *gin.Context, st *auth.Claims, user githubUser) {
	oneTime, err := h.signer.Sign(auth.Claims{
		Typ:       auth.TypCode,
		Web:       true,
		GitHubID:  user.ID,
		Login:     user.Login,
		Challenge: st.Challenge,
		Exp:       time.Now().Add(codeTTL),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "code"})
		return
	}
	h.renderWebAuthBridge(c, oneTime, st.CLIState)
}
