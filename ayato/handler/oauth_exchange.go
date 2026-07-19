package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth"
)

type codeExchangeRequest struct {
	Code         string `json:"code"`
	CodeVerifier string `json:"code_verifier"`
}

func (h *AuthHandler) redeemCode(c *gin.Context, tokenType string) (*auth.Claims, bool) {
	if h.signer == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth not configured"})
		return nil, false
	}
	var request codeExchangeRequest
	if err := c.ShouldBindJSON(&request); err != nil ||
		request.Code == "" || request.CodeVerifier == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return nil, false
	}
	claims, err := h.signer.VerifyTyp(request.Code, tokenType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or used code"})
		return nil, false
	}
	if !auth.VerifyPKCE(request.CodeVerifier, claims.Challenge) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pkce verification failed"})
		return nil, false
	}
	if !h.admins.IsAdmin(claims.GitHubID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not allowed"})
		return nil, false
	}
	if !h.consumeCode(c, request.Code, claims) {
		return nil, false
	}
	return claims, true
}

func (h *AuthHandler) consumeCode(c *gin.Context, code string, claims *auth.Claims) bool {
	if h.replay == nil {
		return true
	}
	firstUse, err := h.replay.Consume(auth.HashHex(code), time.Until(claims.Exp))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token"})
		return false
	}
	if !firstUse {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or used code"})
		return false
	}
	return true
}

func (h *AuthHandler) CLIExchangeHandler(c *gin.Context) {
	claims, ok := h.redeemCode(c, auth.TypCodeCLI)
	if !ok {
		return
	}
	access, refresh, expiresIn, err := h.issueAccessRefresh(
		claims.GitHubID,
		claims.Login,
		"cli",
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token":         access,
		"refresh_token": refresh,
		"expires_in":    expiresIn,
		"login":         claims.Login,
		"id":            claims.GitHubID,
	})
}

func (h *AuthHandler) WebExchangeHandler(c *gin.Context) {
	claims, ok := h.redeemCode(c, auth.TypCodeWeb)
	if !ok {
		return
	}
	jti, err := auth.NewJTI()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token"})
		return
	}
	token, err := h.signer.Sign(auth.Claims{
		Typ:      auth.TypBearer,
		GitHubID: claims.GitHubID,
		Login:    claims.Login,
		Name:     "web",
		JTI:      jti,
		Exp:      time.Now().Add(bearerTTL),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"login": claims.Login,
		"id":    claims.GitHubID,
	})
}
