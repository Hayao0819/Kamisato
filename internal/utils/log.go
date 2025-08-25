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

var clogLevels = map[slog.Level]string{
	slog.LevelDebug: "DEBUG",
	slog.LevelInfo:  "INFO ",
	slog.LevelWarn:  "WARN ",
	slog.LevelError: "ERROR",
}

var clogLevelFormatter = func(level slog.Level) string {
	if v, ok := clogLevels[level]; ok {
		return v
	}
	return level.String()
}

func UseColorLog(level slog.Level) {
	tmpl, err := template.New("default").Parse(clogTemplate)
	if err != nil {
		panic(err)
	}

	h := clog.New(
		clog.WithColor(true),
		clog.WithLevel(level),
		clog.WithSource(true),
		clog.WithLevelFormatter(clogLevelFormatter),
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
