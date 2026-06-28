package handler

import (
	"net/http"
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

// MeHandler reports the current session identity for the SPA.
func (h *Handler) MeHandler(c *gin.Context) {
	if h.signer == nil {
		c.JSON(http.StatusOK, gin.H{"authenticated": false})
		return
	}
	sid, err := c.Cookie(h.cfg.Auth.CookieName())
	if err != nil || sid == "" {
		c.JSON(http.StatusOK, gin.H{"authenticated": false})
		return
	}
	claims, err := h.signer.VerifyTyp(sid, auth.TypSession)
	if err != nil || !h.s.IsAdmin(claims.GitHubID) {
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
