package middleware

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/Hayao0819/Kamisato/conf"
	"github.com/gin-gonic/gin"
)

type Middleware struct {
	cfg *conf.AyatoConfig
}

func NewMiddleware(cfg *conf.AyatoConfig) *Middleware {
	return &Middleware{
		cfg: cfg,
	}
}

func (m *Middleware) BasicAuth(c *gin.Context) {
	const basicPrefix = "Basic "
	auth := c.GetHeader("Authorization")
	if !strings.HasPrefix(auth, basicPrefix) {
		c.Header("WWW-Authenticate", `Basic realm="Restricted"`)
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	username := m.cfg.Username
	password := m.cfg.Password
	if username == "" || password == "" {
		c.Header("WWW-Authenticate", `Basic realm="Restricted"`)
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// Decode the base64 encoded string
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
