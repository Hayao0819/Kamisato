package utils

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/m-mizutani/clog"
	slogGorm "github.com/orandin/slog-gorm"
	sloggin "github.com/samber/slog-gin"
	gormlogger "gorm.io/gorm/logger"
)

func UseColorLog(level slog.Level) {
	h := clog.New(clog.WithColor(true), clog.WithLevel(level))
	l := slog.New(h)
	slog.SetDefault(l)
}

func GinLog() gin.HandlerFunc {
	config := sloggin.Config{
		DefaultLevel: slog.LevelDebug,
	}
	return sloggin.NewWithConfig(slog.Default(), config)
}

func GormLog() gormlogger.Interface {
	gormLogger := slogGorm.New()
	return gormLogger
}
