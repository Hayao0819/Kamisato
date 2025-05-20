package utils

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/m-mizutani/clog"
	sloggin "github.com/samber/slog-gin"
)

func UseColorLog(level slog.Level) {
	h := clog.New(clog.WithColor(true), clog.WithLevel(level))
	l := slog.New(h)
	slog.SetDefault(l)
}

func Gin() gin.HandlerFunc {
	config := sloggin.Config{
		DefaultLevel: slog.LevelDebug,
	}
	return sloggin.NewWithConfig(slog.Default(), config)
}
