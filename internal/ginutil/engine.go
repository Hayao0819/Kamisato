package ginutil

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	sloggin "github.com/samber/slog-gin"
)

// NewEngine returns the engine every server starts from: panic recovery, slog
// request logging, and no trusted proxies so a spoofed X-Forwarded-For cannot
// reach ClientIP; servers that accept proxies opt back in via SetTrustedProxies.
func NewEngine() *gin.Engine {
	e := gin.New()
	e.Use(
		gin.Recovery(),
		sloggin.NewWithConfig(slog.Default(), sloggin.Config{DefaultLevel: slog.LevelDebug, HandleGinDebug: true}),
	)
	// A nil proxy list cannot be an invalid CIDR, so the error is unreachable.
	_ = e.SetTrustedProxies(nil)
	return e
}

// NewServer returns an http.Server with the shared defaults. No ReadTimeout or
// WriteTimeout: large bounded uploads, repository downloads, and SSE streams
// can legitimately be long-lived.
func NewServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}
