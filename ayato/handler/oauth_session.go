package handler

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/gin-gonic/gin"
)

// consumeCode records the presented code as spent so it cannot be replayed. It
// writes the error response and returns false when the code was already redeemed
// (or the guard errors); with no guard wired it allows the exchange. The code id
// is the hash of the code itself, keyed with the code's remaining lifetime so the
// entry self-evicts once the code would have expired.
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

// CLI PKCE exchange. The code is a stateless signed token; a kv-backed one-time
// guard (h.replay, when wired) records its id on first redemption with a TTL equal
// to the code's remaining life, so a second exchange within the 60s window is
// rejected as used. The "used" set lives in the shared kv, not process memory, so
// ayato stays stateless. The bound PKCE verifier still proves possession.
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

	// The PKCE challenge ayaka registered at /cli/start rides state -> code, so the
	// presented verifier proves possession. Pinning TypCodeCLI stops a web-bearer
	// code from being redeemed for the longer-lived CLI token.
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

	// A new login gets a short-lived access token plus a refresh token; the client
	// silently refreshes when the access token expires. Older long-lived tokens keep
	// working until their own expiry (backward compatible).
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
	// Logout mutates session state, so gate it against CSRF and fail closed:
	// accept only a proven same-origin caller. A request that presents no
	// same-origin signal at all (no Sec-Fetch-Site and no matching Origin/Referer)
	// is rejected rather than allowed through.
	if !h.sameOriginRequest(c) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	// Bearer-mode logout: individually revoke the presented web (or CLI) token so
	// it cannot be replayed before its TTL. Best-effort — cookie-mode logout is
	// handled by clearing the cookie below, and the signed session token has no
	// jti and simply expires.
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

// sameOriginRequest is the fail-closed CSRF gate for state-changing browser
// endpoints. Sec-Fetch-Site, when the browser sends it, is authoritative. When
// it is absent, fall back to requiring the Origin/Referer host to equal the
// request host, so a genuine same-origin call still passes. A request carrying
// neither signal (or a cross-site one) is rejected.
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

// hostOfURL returns the host[:port] of a URL, or "" if it cannot be parsed.
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
