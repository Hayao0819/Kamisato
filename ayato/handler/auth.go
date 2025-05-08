package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func LoginHandler(c *gin.Context) {
	username, password, ok := c.Request.BasicAuth()
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if username == config.Username && password == config.Password {
		c.String(http.StatusOK, "Login successful")
		return
	}

	c.AbortWithStatus(http.StatusUnauthorized)
}

func LogoutHandler(c *gin.Context) {
	c.String(http.StatusOK, "Logout successful")
}
