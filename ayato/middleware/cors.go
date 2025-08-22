package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Cors is a Gin middleware for CORS settings.
func (m *Middleware) Cors() gin.HandlerFunc {
	c := cors.DefaultConfig()
	c.AllowAllOrigins = true
	c.AllowMethods = []string{
		"GET",
		"PUT",
		"DELETE",
	}
	return cors.New(c)
}
