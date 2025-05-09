package middleware

import (
	"context"
	"net/http"

	"github.com/Hayao0819/Kamisato/conf"
	"github.com/gin-gonic/gin"
)

type contextKey string

const configKey = contextKey("config")

func Config() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := conf.LoadAyatoConfig()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "failed to load configuration",
			})
			return
		}

		ctx := context.WithValue(c.Request.Context(), configKey, cfg)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func GetConfig(c *gin.Context) *conf.AyatoConfig {
	cfg, ok := c.Request.Context().Value(configKey).(*conf.AyatoConfig)
	if !ok {
		return nil
	}
	return cfg
}
