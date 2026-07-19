package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/httpsecurity"
)

// MeHandler accepts a cookie session or bearer token and re-checks the admin
// allowlist before reporting the requester as authenticated.
func (h *AuthHandler) MeHandler(c *gin.Context) {
	if h.signer == nil {
		c.JSON(http.StatusOK, gin.H{"authenticated": false})
		return
	}
	if sid, err := c.Cookie(h.cfg.Auth.CookieName()); err == nil && sid != "" {
		claims, verifyErr := h.signer.VerifyTyp(sid, auth.TypSession)
		if verifyErr == nil && h.admins.IsAdmin(claims.GitHubID) {
			c.JSON(http.StatusOK, gin.H{
				"authenticated": true,
				"id":            claims.GitHubID,
				"login":         claims.Login,
			})
			return
		}
	}
	if authz := c.GetHeader("Authorization"); strings.HasPrefix(authz, "Bearer ") {
		token := strings.TrimPrefix(authz, "Bearer ")
		for _, tokenType := range []string{auth.TypBearer, auth.TypCLI} {
			claims, verifyErr := h.signer.VerifyTyp(token, tokenType)
			if verifyErr != nil || !h.admins.IsAdmin(claims.GitHubID) {
				continue
			}
			revoked, revokeErr := h.claimsRevoked(claims)
			if revokeErr != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"authenticated": false,
					"error":         "revocation store",
				})
				return
			}
			if !revoked {
				c.JSON(http.StatusOK, gin.H{
					"authenticated": true,
					"id":            claims.GitHubID,
					"login":         claims.Login,
				})
				return
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"authenticated": false})
}

func (h *AuthHandler) LogoutHandler(c *gin.Context) {
	if !h.sameOriginRequest(c) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	if h.signer != nil {
		h.revokePresentedAccess(c)
		if c.IsAborted() {
			return
		}
	}
	if h.cfg != nil {
		scheme, _ := h.externalBase(c)
		h.setSessionCookie(c, "", scheme == "https", -1)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AuthHandler) revokePresentedAccess(c *gin.Context) {
	authz := c.GetHeader("Authorization")
	if !strings.HasPrefix(authz, "Bearer ") {
		return
	}
	token := strings.TrimPrefix(authz, "Bearer ")
	for _, tokenType := range []string{auth.TypBearer, auth.TypCLI} {
		claims, err := h.signer.VerifyTyp(token, tokenType)
		if err != nil || claims.JTI == "" {
			continue
		}
		if err := h.revoker.Revoke(claims.JTI, time.Until(claims.Exp)); err != nil {
			c.Abort()
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "revocation store"})
			return
		}
		if tokenType == auth.TypCLI && claims.SessionID != "" {
			if err := h.revoker.RevokeSession(claims.SessionID, h.refreshTTL()); err != nil {
				c.Abort()
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "revocation store"})
			}
		}
		return
	}
}

func (h *AuthHandler) sameOriginRequest(c *gin.Context) bool {
	var allowed []string
	if h.cfg != nil {
		allowed = append(allowed, h.cfg.Auth.SelfOrigin, h.cfg.Auth.PublicOrigin)
	}
	if allowedOrigin(allowed) == "" {
		_, base := h.externalBase(c)
		allowed = append(allowed, base)
	}
	return httpsecurity.SameOrigin(c.Request, allowed...)
}

func allowedOrigin(origins []string) string {
	for _, origin := range origins {
		if httpsecurity.Origin(origin) != "" {
			return origin
		}
	}
	return ""
}
