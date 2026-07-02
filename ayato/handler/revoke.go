package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/gin-gonic/gin"
)

// RevokeCLIHandler denylists the presented CLI token by its jti, invalidating
// that one token server-side before its TTL. Possessing a validly-signed token
// is the authorization (as with logout), and the signature check stops a caller
// from denylisting arbitrary jtis, so no admin middleware guards this route.
func (h *Handler) RevokeCLIHandler(c *gin.Context) {
	if h.signer == nil || h.denylist == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "revocation not configured"})
		return
	}
	authz := c.GetHeader("Authorization")
	if !strings.HasPrefix(authz, "Bearer ") {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "bearer token required"})
		return
	}
	claims, err := h.signer.VerifyTyp(strings.TrimPrefix(authz, "Bearer "), auth.TypCLI)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	if claims.JTI == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "token has no jti; re-login to mint a revocable token"})
		return
	}
	if err := h.denylist.Revoke(claims.JTI, time.Until(claims.Exp)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "revoke failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
