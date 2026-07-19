package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth"
)

func sessionFamilyID(claims *auth.Claims) string {
	if claims == nil {
		return ""
	}
	if claims.SessionID != "" {
		return claims.SessionID
	}
	return claims.JTI
}

func (h *Handler) claimsRevoked(claims *auth.Claims) (bool, error) {
	if claims.JTI != "" {
		revoked, err := h.s.IsRevoked(claims.JTI)
		if err != nil || revoked {
			return revoked, err
		}
	}
	if claims.SessionID != "" {
		return h.s.IsSessionRevoked(claims.SessionID)
	}
	return false, nil
}

// RevokeCLIHandler denylists the presented tokens by jti before their TTL: the Bearer
// access token and, if supplied, the refresh_token, so logout kills both halves.
// Possessing a validly-signed token is the authorization (the signature stops a caller
// denylisting arbitrary jtis), so no admin middleware guards this route; a refresh
// token alone suffices once the access token has expired.
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
	families := make(map[string]time.Duration, 2)
	for _, claims := range []*auth.Claims{access, refresh} {
		if claims == nil {
			continue
		}
		family := sessionFamilyID(claims)
		if family == "" {
			continue
		}
		ttl := h.refreshTTL()
		if remaining := time.Until(claims.Exp); remaining > ttl {
			ttl = remaining
		}
		if current := families[family]; ttl > current {
			families[family] = ttl
		}
	}
	for family, ttl := range families {
		if err := h.s.RevokeSession(family, ttl); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "revoke failed"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
