// Package weblog holds the gin and gorm logging adapters. It lives apart from
// internal/utils so that pure libraries depending on utils do not transitively
// pull in the gin web framework and the gorm ORM.
package weblog

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	slogGorm "github.com/orandin/slog-gorm"
	sloggin "github.com/samber/slog-gin"
	gormlogger "gorm.io/gorm/logger"
)

// GinLog returns a gin middleware that logs requests through the default slog logger.
func GinLog() gin.HandlerFunc {
	config := sloggin.Config{
		DefaultLevel:   slog.LevelDebug,
		HandleGinDebug: true,
	}
	return sloggin.NewWithConfig(slog.Default(), config)
}

// GormLog returns a gorm logger backed by the default slog logger.
func GormLog() gormlogger.Interface {
	return slogGorm.New()
}
