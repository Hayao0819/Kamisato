package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Readiness is the process state observed by the readiness endpoint. The router
// depends on this narrow port rather than the lifecycle implementation.
type Readiness interface {
	Ready() bool
}

type routeConfig struct {
	readiness Readiness
}

type RouteOption func(*routeConfig)

// WithReadiness makes /ready reflect the process lifecycle. Without it, the
// endpoint remains ready for isolated router tests and embedded users.
func WithReadiness(readiness Readiness) RouteOption {
	return func(config *routeConfig) {
		config.readiness = readiness
	}
}

func setHealthRoutes(engine *gin.Engine, readiness Readiness) {
	engine.GET("/health", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, "ok")
	})
	engine.GET("/ready", func(ctx *gin.Context) {
		ready := readiness == nil || readiness.Ready()
		status := http.StatusOK
		if !ready {
			status = http.StatusServiceUnavailable
		}
		ctx.JSON(status, gin.H{"ready": ready})
	})
}
