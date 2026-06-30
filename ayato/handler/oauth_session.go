package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/gin-gonic/gin"
)

// CLI PKCE exchange. The code is a stateless signed token with no stored "used"
// set, so it is replay-limited, not single-use: its 60s TTL and the bound PKCE
// verifier are what constrain replay within that window.
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
	// presented verifier proves possession.
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

// Web-bearer PKCE exchange. The code must be a web code (Web=true) so a CLI/cookie
// code can never be redeemed for a bearer token here.
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
	// Defense-in-depth vs logout CSRF: reject cross-site callers (non-browser
	// callers send no Sec-Fetch-Site).
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
