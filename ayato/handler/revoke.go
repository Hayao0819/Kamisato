package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/gin-gonic/gin"
)

// RevokeCLIHandler denylists the presented tokens by their jti, invalidating them
// server-side before their TTL. It revokes the Bearer access token and, when a
// refresh_token is supplied in the body, that too — so a logout kills both halves
// of the session. Possessing a validly-signed token is the authorization (as with
// logout), and the signature check stops a caller from denylisting arbitrary jtis,
// so no admin middleware guards this route. A valid refresh token alone suffices,
// which matters once the short-lived access token has already expired.
func (h *Handler) RevokeCLIHandler(c *gin.Context) {
	if h.signer == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "revocation not configured"})
		return
	}
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = c.ShouldBindJSON(&body) // body is optional (access-only revoke)

	var access, refresh *auth.Claims
	if authz := c.GetHeader("Authorization"); strings.HasPrefix(authz, "Bearer ") {
		if claims, err := h.signer.VerifyTyp(strings.TrimPrefix(authz, "Bearer "), auth.TypCLI); err == nil {
			access = claims
		}
	}
	if body.RefreshToken != "" {
		if claims, err := h.signer.VerifyTyp(body.RefreshToken, auth.TypRefresh); err == nil {
			refresh = claims
		}
	}

	// No validly-signed token presented at all: unauthenticated.
	if access == nil && refresh == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	// A revocable token must carry a jti; a pre-jti access token with no refresh
	// fallback cannot be individually revoked.
	if (access == nil || access.JTI == "") && (refresh == nil || refresh.JTI == "") {
		c.JSON(http.StatusConflict, gin.H{"error": "token has no jti; re-login to mint a revocable token"})
		return
	}

	if access != nil && access.JTI != "" {
		if err := h.s.Revoke(access.JTI, time.Until(access.Exp)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "revoke failed"})
			return
		}
	}
	if refresh != nil && refresh.JTI != "" {
		if err := h.s.Revoke(refresh.JTI, time.Until(refresh.Exp)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "revoke failed"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
