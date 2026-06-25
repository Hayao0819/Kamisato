package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
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

// oauthEnabled reports whether the GitHub login flow is configured.
func (h *Handler) oauthEnabled() bool {
	return h.signer != nil && h.allow != nil && h.cfg != nil && h.cfg.Auth.GitHub.ClientID != "" && h.cfg.Auth.GitHub.ClientSecret != ""
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

// GitHubLoginHandler starts a WEB "Sign in with GitHub" flow: it mints a signed
// state token (carrying the browser binding) and redirects to GitHub's consent
// page. No server-side state is written — the signed token IS the state.
func (h *Handler) GitHubLoginHandler(c *gin.Context) {
	if !h.oauthEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "github login not configured"})
		return
	}
	// Bind the flow to THIS browser: a random nonce travels in a host-only
	// SameSite=Lax cookie and only its hash is signed into the state. The callback
	// requires the cookie to match, defeating login-CSRF / session fixation
	// (RFC 6749 §10.12).
	nonce, err := auth.NewState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "state"})
		return
	}
	state, err := h.signer.Sign(auth.Claims{
		Typ:     auth.TypState,
		CLI:     false,
		Binding: auth.HashHex(nonce),
		Exp:     time.Now().Add(stateTTL),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "state"})
		return
	}
	scheme, _ := h.externalBase(c)
	h.setOAuthStateCookie(c, nonce, scheme == "https")
	c.Redirect(http.StatusFound, h.oauthConfig(c).AuthCodeURL(state))
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

// CLIStartHandler starts a CLI flow. ayaka opens this URL in the user's browser
// with the loopback port, a PKCE S256 challenge, and a state. The loopback URL
// is reconstructed server-side from the integer port (never a full URL); ayaka's
// original state is carried inside the signed state token so the callback can
// echo it back unchanged. The signed token IS the state sent to GitHub.
func (h *Handler) CLIStartHandler(c *gin.Context) {
	if !h.oauthEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "github login not configured"})
		return
	}
	portStr := c.Query("port")
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 || strconv.Itoa(port) != portStr {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid port"})
		return
	}
	challenge := c.Query("challenge")
	if challenge == "" || len(challenge) > 256 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing or oversized challenge"})
		return
	}
	cliState := c.Query("state")
	if len(cliState) > 256 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "oversized state"})
		return
	}
	if cliState == "" {
		var serr error
		if cliState, serr = auth.NewState(); serr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "state"})
			return
		}
	}
	state, err := h.signer.Sign(auth.Claims{
		Typ:       auth.TypState,
		CLI:       true,
		Port:      port,
		Challenge: challenge,
		CLIState:  cliState,
		Exp:       time.Now().Add(stateTTL),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "state"})
		return
	}
	c.Redirect(http.StatusFound, h.oauthConfig(c).AuthCodeURL(state))
}

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

	// Web flow: the state must be redeemed by the SAME browser that began it.
	// The binding cookie is single-use, so clear it regardless of the outcome.
	if !st.CLI {
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
	if !h.allow.Has(user.ID) {
		slog.Warn("github login denied (not allowlisted)", "github_id", user.ID, "login", user.Login)
		c.JSON(http.StatusForbidden, gin.H{"error": "not allowed"})
		return
	}

	if st.CLI {
		h.finishCLILogin(c, st, user)
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

// CLIExchangeHandler completes the CLI PKCE exchange. ayaka POSTs the one-time
// code plus its PKCE verifier over its DIRECT ayato connection (so the token is
// never placed in a URL). On success it returns a freshly signed CLI token. The
// one-time code is single-use by construction: its 60s TTL plus the PKCE
// verifier (which proves one-time possession) bound replay; no stored "used" set.
func (h *Handler) CLIExchangeHandler(c *gin.Context) {
	if h.signer == nil || h.allow == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth not configured"})
		return
	}
	var body struct {
		Code         string `json:"code"`
		CodeVerifier string `json:"code_verifier"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Code == "" || body.CodeVerifier == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Verify the signed one-time code. The PKCE challenge that ayaka registered at
	// /cli/start travels state -> code, so the presented verifier proves
	// possession of that challenge.
	rec, err := h.signer.VerifyTyp(body.Code, auth.TypCode)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or used code"})
		return
	}

	if !auth.VerifyPKCE(body.CodeVerifier, rec.Challenge) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pkce verification failed"})
		return
	}

	// Re-check allowlist at exchange time (fail-closed).
	if !h.allow.Has(rec.GitHubID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not allowed"})
		return
	}

	token, err := h.signer.Sign(auth.Claims{
		Typ:      auth.TypCLI,
		GitHubID: rec.GitHubID,
		Login:    rec.Login,
		Name:     "cli",
		Exp:      time.Now().Add(tokenTTL),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "login": rec.Login, "id": rec.GitHubID})
}

// MeHandler reports the current session identity for the SPA.
func (h *Handler) MeHandler(c *gin.Context) {
	if h.signer == nil || h.allow == nil {
		c.JSON(http.StatusOK, gin.H{"authenticated": false})
		return
	}
	sid, err := c.Cookie(h.cfg.Auth.CookieName())
	if err != nil || sid == "" {
		c.JSON(http.StatusOK, gin.H{"authenticated": false})
		return
	}
	claims, err := h.signer.VerifyTyp(sid, auth.TypSession)
	if err != nil || !h.allow.Has(claims.GitHubID) {
		c.JSON(http.StatusOK, gin.H{"authenticated": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{"authenticated": true, "id": claims.GitHubID, "login": claims.Login})
}

// LogoutHandler clears the session cookie. Sessions are stateless, so there is
// no server-side record to delete; clearing the cookie ends the session for the
// browser, and the signed token would otherwise expire by TTL.
func (h *Handler) LogoutHandler(c *gin.Context) {
	// Defense-in-depth against logout CSRF: reject a cross-site caller. The SPA
	// logs out via same-origin fetch; non-browser callers send no Sec-Fetch-Site.
	if sfs := c.GetHeader("Sec-Fetch-Site"); sfs != "" && sfs != "same-origin" {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	if h.cfg != nil {
		scheme, _ := h.externalBase(c)
		h.setSessionCookie(c, "", scheme == "https", -1)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---- admin endpoints ----

// AdminsListHandler lists allowlisted admins.
func (h *Handler) AdminsListHandler(c *gin.Context) {
	admins, err := h.allow.ListAllowed()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list"})
		return
	}
	if admins == nil {
		admins = []auth.AllowedAdmin{}
	}
	c.JSON(http.StatusOK, gin.H{"admins": admins})
}

// AdminsAddHandler adds an admin by numeric id, or by GitHub login (resolved to
// an id via the GitHub API).
func (h *Handler) AdminsAddHandler(c *gin.Context) {
	var body struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	id, login := body.ID, body.Login
	if id <= 0 {
		if login == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id or login required"})
			return
		}
		resolved, rerr := resolveGitHubLogin(c.Request.Context(), login)
		if rerr != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "could not resolve login"})
			return
		}
		id, login = resolved.ID, resolved.Login
	}
	if err := h.allow.Add(id, login); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "add"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "login": login})
}

// AdminsRemoveHandler removes an admin by numeric id.
func (h *Handler) AdminsRemoveHandler(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.allow.Remove(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "remove"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// resolveGitHubLogin looks up a public GitHub user by login to get the numeric
// id (no auth required for public profiles).
func resolveGitHubLogin(ctx context.Context, login string) (githubUser, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	u := "https://api.github.com/users/" + url.PathEscape(login)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return githubUser{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return githubUser{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return githubUser{}, errors.New("github users lookup non-200")
	}
	var gu githubUser
	if err := json.NewDecoder(resp.Body).Decode(&gu); err != nil || gu.ID == 0 {
		return githubUser{}, errors.New("github users decode")
	}
	return gu, nil
}
