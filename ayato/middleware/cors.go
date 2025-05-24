package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

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
