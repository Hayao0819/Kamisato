package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/gin-gonic/gin"
)

// CLIExchangeHandler completes the CLI PKCE exchange. ayaka POSTs the one-time
// code plus its PKCE verifier over its DIRECT ayato connection (so the token is
// never placed in a URL). On success it returns a freshly signed CLI token. The
// one-time code is single-use by construction: its 60s TTL plus the PKCE
// verifier (which proves one-time possession) bound replay; no stored "used" set.
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
	if !h.s.IsAdmin(rec.GitHubID) {
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

// WebExchangeHandler completes the web-bearer PKCE exchange. The static SPA POSTs
// the one-time code (delivered to it via postMessage) plus its PKCE verifier and
// receives a short-lived bearer token it presents as Authorization: Bearer. As in
// the CLI exchange, PKCE possession plus the code's 60s TTL bound replay. The code
// must be a web code (Web=true) so a CLI/cookie code can never be redeemed for a
// web bearer token here.
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
	rec, err := h.signer.VerifyTyp(body.Code, auth.TypCode)
	if err != nil || !rec.Web {
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
	token, err := h.signer.Sign(auth.Claims{
		Typ:      auth.TypBearer,
		GitHubID: rec.GitHubID,
		Login:    rec.Login,
		Name:     "web",
		Exp:      time.Now().Add(bearerTTL),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "login": rec.Login, "id": rec.GitHubID})
}

// MeHandler reports the signed-in identity. It accepts the cookie session
// (same-origin/BFF mode) or a Bearer token (cross-origin bearer mode); the
// allowlist is re-checked so a de-allowlisted id reads as unauthenticated.
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
				c.JSON(http.StatusOK, gin.H{"authenticated": true, "id": claims.GitHubID, "login": claims.Login})
				return
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"authenticated": false})
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
