package handler

import (
	"net/http"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/gin-gonic/gin"
)

func (h *Handler) accessTTL() time.Duration {
	if h.cfg != nil {
		return h.cfg.Auth.AccessTokenTTL()
	}
	return time.Hour
}

func (h *Handler) refreshTTL() time.Duration {
	if h.cfg != nil {
		return h.cfg.Auth.RefreshTokenTTL()
	}
	return 30 * 24 * time.Hour
}

// issueAccessRefresh mints a short-lived access token paired with a long-lived
// refresh token, each with its own jti so either can be revoked independently.
func (h *Handler) issueAccessRefresh(githubID int64, login, name string) (access, refresh string, expiresIn int, err error) {
	accessJTI, err := auth.NewJTI()
	if err != nil {
		return "", "", 0, err
	}
	accessTTL := h.accessTTL()
	access, err = h.signer.Sign(auth.Claims{
		Typ:      auth.TypCLI,
		GitHubID: githubID,
		Login:    login,
		Name:     name,
		JTI:      accessJTI,
		Exp:      time.Now().Add(accessTTL),
	})
	if err != nil {
		return "", "", 0, err
	}
	refreshJTI, err := auth.NewJTI()
	if err != nil {
		return "", "", 0, err
	}
	refresh, err = h.signer.Sign(auth.Claims{
		Typ:      auth.TypRefresh,
		GitHubID: githubID,
		Login:    login,
		JTI:      refreshJTI,
		Exp:      time.Now().Add(h.refreshTTL()),
	})
	if err != nil {
		return "", "", 0, err
	}
	return access, refresh, int(accessTTL / time.Second), nil
}

// RefreshHandler trades a valid, non-revoked refresh token for a fresh access token
// and rotates the refresh token: the old jti is denylisted so a stolen copy cannot be
// reused (RFC 6749 §10.4). The signature is the authorization, so no admin middleware guards it.
func (h *Handler) RefreshHandler(c *gin.Context) {
	if h.signer == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth not configured"})
		return
	}
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.RefreshToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// An expired or wrong-type token fails here; the client must re-login.
	rec, err := h.signer.VerifyTyp(body.RefreshToken, auth.TypRefresh)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_grant"})
		return
	}
	if rec.JTI != "" && h.s.IsRevoked(rec.JTI) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_grant"})
		return
	}
	// Re-check the allowlist so a de-allowlisted id cannot refresh (fail-closed).
	if !h.s.IsAdmin(rec.GitHubID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not allowed"})
		return
	}

	// Rotate: denylist the consumed refresh token before issuing its replacement.
	// Best-effort — without a denylist the old refresh token stays valid until expiry.
	if rec.JTI != "" {
		_ = h.s.Revoke(rec.JTI, time.Until(rec.Exp))
	}

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
