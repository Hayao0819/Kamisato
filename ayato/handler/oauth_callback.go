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

// GitHubCallbackHandler completes both web and CLI flows: it verifies the signed
// state, exchanges the code for an access token, resolves the GitHub identity,
// applies the fail-closed allowlist check, and then either sets a signed session
// cookie (web) or redirects a one-time CODE to the loopback (CLI). The GitHub
// access token is discarded once identity is resolved.
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

	// Cookie web flow: the state must be redeemed by the SAME browser that began
	// it. The binding cookie is single-use, so clear it regardless of the outcome.
	// The CLI and web-bearer flows bind via PKCE instead, so they skip this check.
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

// resolveGitHubUser exchanges the code for a token, calls GET /user, and
// discards the token. Any error fails closed (returns ok=false).
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
	// The access token's only use is this single /user call; it is never stored
	// (no session/token record keeps it), so it is discarded when this function
	// returns.
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

// finishWebLogin mints a fresh signed session token (fixation defense: a new
// envelope is issued every login, no stored sid to keep), sets a host-only
// HttpOnly SameSite=Lax cookie (Secure when the external scheme is https), and
// redirects to "/" (never an attacker-supplied target).
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

// finishCLILogin mints a one-time signed CODE (never a token) and redirects it to
// the server-reconstructed loopback, echoing ayaka's ORIGINAL state back so its
// own state-equality check passes. No session cookie is set for CLI.
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
	// Reconstruct the loopback URL server-side from the integer port only and
	// echo ayaka's original state (carried in the signed state token).
	target := url.URL{
		Scheme:   "http",
		Host:     "127.0.0.1:" + strconv.Itoa(st.Port),
		Path:     "/",
		RawQuery: url.Values{"code": {oneTime}, "state": {st.CLIState}}.Encode(),
	}
	c.Redirect(http.StatusFound, target.String())
}

// finishWebBearerLogin mints a one-time signed CODE (Web=true) and returns it to
// the SPA through the postMessage bridge page — never a token. The SPA redeems
// the code at /auth/web/exchange with its PKCE verifier. No session cookie is set.
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
