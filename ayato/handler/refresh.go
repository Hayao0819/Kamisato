package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth"
)

func (h *AuthHandler) accessTTL() time.Duration {
	if h.cfg != nil {
		return h.cfg.Auth.AccessTokenTTL()
	}
	return time.Hour
}

func (h *AuthHandler) refreshTTL() time.Duration {
	if h.cfg != nil {
		return h.cfg.Auth.RefreshTokenTTL()
	}
	return 30 * 24 * time.Hour
}

// issueAccessRefresh creates a CLI session family.
func (h *AuthHandler) issueAccessRefresh(githubID int64, login, name string) (access, refresh string, expiresIn int, err error) {
	sessionID, err := auth.NewJTI()
	if err != nil {
		return "", "", 0, err
	}
	return h.issueAccessRefreshForSession(githubID, login, name, sessionID, time.Now().Add(h.refreshTTL()))
}

func (h *AuthHandler) issueAccessRefreshForSession(githubID int64, login, name, sessionID string, refreshExpiresAt time.Time) (access, refresh string, expiresIn int, err error) {
	if sessionID == "" || !refreshExpiresAt.After(time.Now()) {
		return "", "", 0, auth.ErrExpired
	}
	accessJTI, err := auth.NewJTI()
	if err != nil {
		return "", "", 0, err
	}
	accessTTL := h.accessTTL()
	access, err = h.signer.Sign(auth.Claims{
		Typ:       auth.TypCLI,
		GitHubID:  githubID,
		Login:     login,
		Name:      name,
		JTI:       accessJTI,
		SessionID: sessionID,
		Exp:       time.Now().Add(accessTTL),
	})
	if err != nil {
		return "", "", 0, err
	}
	refreshJTI, err := auth.NewJTI()
	if err != nil {
		return "", "", 0, err
	}
	refresh, err = h.signer.Sign(auth.Claims{
		Typ:       auth.TypRefresh,
		GitHubID:  githubID,
		Login:     login,
		JTI:       refreshJTI,
		SessionID: sessionID,
		Exp:       refreshExpiresAt,
	})
	if err != nil {
		return "", "", 0, err
	}
	return access, refresh, int(accessTTL / time.Second), nil
}

// RefreshHandler trades a valid, non-revoked refresh token for a fresh access token
// and rotates the refresh token: the old jti is denylisted so a stolen copy cannot be
// reused (RFC 6749 §10.4). The signature is the authorization, so no admin middleware guards it.
func (h *AuthHandler) RefreshHandler(c *gin.Context) {
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
	if rec.JTI == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_grant"})
		return
	}
	sessionID := sessionFamilyID(rec)
	sessionRevoked, err := h.revoker.IsSessionRevoked(sessionID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "refresh unavailable"})
		return
	}
	if sessionRevoked {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_grant"})
		return
	}
	// Re-check the allowlist so a de-allowlisted id cannot refresh (fail-closed).
	if !h.admins.IsAdmin(rec.GitHubID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not allowed"})
		return
	}

	access, refresh, expiresIn, err := h.issueAccessRefreshForSession(rec.GitHubID, rec.Login, "cli", sessionID, rec.Exp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token"})
		return
	}
	consumed, err := h.revoker.ConsumeRefreshToken(rec.JTI, time.Until(rec.Exp))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "refresh unavailable"})
		return
	}
	if !consumed {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_grant"})
		return
	}
	sessionRevoked, err = h.revoker.IsSessionRevoked(sessionID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "refresh unavailable"})
		return
	}
	if sessionRevoked {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_grant"})
		return
	}
	respondTokenPair(c, access, refresh, expiresIn, rec.Login, rec.GitHubID)
}
