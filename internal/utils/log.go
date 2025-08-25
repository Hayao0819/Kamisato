package utils

import (
	"log/slog"
	"text/template"

	"github.com/gin-gonic/gin"
	"github.com/m-mizutani/clog"
	slogGorm "github.com/orandin/slog-gorm"
	sloggin "github.com/samber/slog-gin"
	gormlogger "gorm.io/gorm/logger"
)

const clogTemplate = `{{.Level}} {{.Message}}{{ if .FileName }} [{{.FileName}}:{{.FileLine}}]{{ end }} `

func UseColorLog(level slog.Level) {
	tmpl, err := template.New("default").Parse(clogTemplate)
	if err != nil {
		panic(err)
	}

	h := clog.New(
		clog.WithColor(true),
		clog.WithLevel(level),
		clog.WithSource(true),
		clog.WithTemplate(tmpl),
	)
	l := slog.New(h)
	slog.SetDefault(l)
}

// func UseLog(level slog.Level) {
// 	h := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
// 		Level: level,
// 	}))
// 	slog.SetDefault(h)
// }

func GinLog() gin.HandlerFunc {
	config := sloggin.Config{
		DefaultLevel:   slog.LevelDebug,
		HandleGinDebug: true,
	}
	return sloggin.NewWithConfig(slog.Default(), config)
}

func GormLog() gormlogger.Interface {
	gormLogger := slogGorm.New()
	return gormLogger
}
