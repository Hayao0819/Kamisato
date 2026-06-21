package middleware

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/gin-gonic/gin"
)

// Middleware provides middleware for authentication, authorization, etc.
type Middleware struct {
	cfg *conf.AyatoConfig
}

func New(cfg *conf.AyatoConfig) *Middleware {
	return &Middleware{
		cfg: cfg,
	}
}

// BasicAuth is a Gin middleware for Basic authentication.
func (m *Middleware) BasicAuth(c *gin.Context) {
	const basicPrefix = "Basic "
	auth := c.GetHeader("Authorization")
	if !strings.HasPrefix(auth, basicPrefix) {
		c.Header("WWW-Authenticate", `Basic realm="Restricted"`)
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	username := m.cfg.Auth.Username
	password := m.cfg.Auth.Password
	// If username and password are empty, skip authentication
	if username == "" && password == "" {
		c.Next()
		return
	}

	payload, err := base64.StdEncoding.DecodeString(auth[len(basicPrefix):])
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	pair := strings.SplitN(string(payload), ":", 2)
	if len(pair) != 2 || pair[0] != username || pair[1] != password {
		c.Header("WWW-Authenticate", `Basic realm="Restricted"`)
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	c.Next()

}
