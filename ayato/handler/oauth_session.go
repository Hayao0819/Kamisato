package handler

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth"
)

// consumeCode marks the code spent so it cannot be replayed (allowed when no guard
// is wired); the entry is keyed by the code hash with a TTL of its remaining lifetime
// so it self-evicts.
func (h *Handler) consumeCode(c *gin.Context, code string, rec *auth.Claims) bool {
	if h.replay == nil {
		return true
	}
	firstUse, err := h.replay.Consume(auth.HashHex(code), time.Until(rec.Exp))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token"})
		return false
	}
	if !firstUse {
		// Same opaque message as an invalid code: a caller cannot tell replay apart.
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or used code"})
		return false
	}
	return true
}

// CLIExchangeHandler performs the CLI PKCE exchange; a kv-backed one-time guard
// (h.replay) rejects a replayed code, and the shared-kv "used" set keeps ayato stateless.
func (h *Handler) CLIExchangeHandler(c *gin.Context) {
	if h.signer == nil {
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

	// The verifier proves possession of the challenge registered at /cli/start; pinning
	// TypCodeCLI stops a web-bearer code from being redeemed for the CLI token.
	rec, err := h.signer.VerifyTyp(body.Code, auth.TypCodeCLI)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or used code"})
		return
	}

	if !auth.VerifyPKCE(body.CodeVerifier, rec.Challenge) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pkce verification failed"})
		return
	}

	// Re-check allowlist at exchange time (fail-closed).
	if !h.s.IsAdmin(rec.GitHubID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not allowed"})
		return
	}

	// One-time guard: redeem this code exactly once (a replay reads as used).
	if !h.consumeCode(c, body.Code, rec) {
		return
	}

	// New logins get a short-lived access token plus refresh token; older long-lived
	// tokens keep working until their own expiry (backward compatible).
	access, refresh, expiresIn, err := h.issueAccessRefresh(rec.GitHubID, rec.Login, "cli")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token":         access,
		"refresh_token": refresh,
		"expires_in":    expiresIn,
		"login":         rec.Login,
		"id":            rec.GitHubID,
	})
}

// Web-bearer PKCE exchange. Pinning TypCodeWeb means a CLI/cookie code can never
// be redeemed for a bearer token here.
func (h *Handler) WebExchangeHandler(c *gin.Context) {
	if h.signer == nil {
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
	rec, err := h.signer.VerifyTyp(body.Code, auth.TypCodeWeb)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or used code"})
		return
	}
	if !auth.VerifyPKCE(body.CodeVerifier, rec.Challenge) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pkce verification failed"})
		return
	}
	// Re-check allowlist at exchange time (fail-closed).
	if !h.s.IsAdmin(rec.GitHubID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not allowed"})
		return
	}
	// One-time guard: redeem this code exactly once (a replay reads as used).
	if !h.consumeCode(c, body.Code, rec) {
		return
	}
	// A jti makes the web bearer individually revocable (logout denylists it),
	// matching the CLI token; without it a leaked bearer lives its full TTL.
	jti, err := auth.NewJTI()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token"})
		return
	}
	token, err := h.signer.Sign(auth.Claims{
		Typ:      auth.TypBearer,
		GitHubID: rec.GitHubID,
		Login:    rec.Login,
		Name:     "web",
		JTI:      jti,
		Exp:      time.Now().Add(bearerTTL),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "login": rec.Login, "id": rec.GitHubID})
}

// Accepts the cookie session or a Bearer token; the allowlist is re-checked so a
// de-allowlisted id reads as unauthenticated.
func (h *Handler) MeHandler(c *gin.Context) {
	if h.signer == nil {
		c.JSON(http.StatusOK, gin.H{"authenticated": false})
		return
	}
	if sid, err := c.Cookie(h.cfg.Auth.CookieName()); err == nil && sid != "" {
		if claims, verr := h.signer.VerifyTyp(sid, auth.TypSession); verr == nil && h.s.IsAdmin(claims.GitHubID) {
			c.JSON(http.StatusOK, gin.H{"authenticated": true, "id": claims.GitHubID, "login": claims.Login})
			return
		}
	}
	if authz := c.GetHeader("Authorization"); strings.HasPrefix(authz, "Bearer ") {
		tok := strings.TrimPrefix(authz, "Bearer ")
		for _, typ := range []string{auth.TypBearer, auth.TypCLI} {
			if claims, verr := h.signer.VerifyTyp(tok, typ); verr == nil && h.s.IsAdmin(claims.GitHubID) {
				if claims.JTI != "" && h.s.IsRevoked(claims.JTI) {
					continue
				}
				c.JSON(http.StatusOK, gin.H{"authenticated": true, "id": claims.GitHubID, "login": claims.Login})
				return
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"authenticated": false})
}

// Sessions are stateless: clearing the cookie ends the session; the signed token
// otherwise expires by TTL.
func (h *Handler) LogoutHandler(c *gin.Context) {
	// Logout mutates session state, so gate it against CSRF: accept only a proven
	// same-origin caller, rejecting a request with no same-origin signal (fail closed).
	if !h.sameOriginRequest(c) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	// Bearer-mode logout: revoke the presented token by jti so it cannot be replayed
	// before its TTL. Best-effort; cookie-mode logout is handled by clearing the cookie below.
	if h.signer != nil {
		if authz := c.GetHeader("Authorization"); strings.HasPrefix(authz, "Bearer ") {
			tok := strings.TrimPrefix(authz, "Bearer ")
			for _, typ := range []string{auth.TypBearer, auth.TypCLI} {
				if claims, verr := h.signer.VerifyTyp(tok, typ); verr == nil && claims.JTI != "" {
					_ = h.s.Revoke(claims.JTI, time.Until(claims.Exp))
					break
				}
			}
		}
	}
	if h.cfg != nil {
		scheme, _ := h.externalBase(c)
		h.setSessionCookie(c, "", scheme == "https", -1)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// sameOriginRequest is the fail-closed CSRF gate: Sec-Fetch-Site is authoritative
// when present, else the Origin/Referer host must equal the request host; a request
// with neither signal is rejected.
func (h *Handler) sameOriginRequest(c *gin.Context) bool {
	if sfs := c.GetHeader("Sec-Fetch-Site"); sfs != "" {
		return sfs == "same-origin"
	}
	origin := c.GetHeader("Origin")
	if origin == "" {
		origin = c.GetHeader("Referer")
	}
	host := hostOfURL(origin)
	return host != "" && strings.EqualFold(host, c.Request.Host)
}

func hostOfURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Host
}
