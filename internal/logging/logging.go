package logging

import (
	"log/slog"
	"os"
	"text/template"

	"github.com/m-mizutani/clog"
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

// Setup installs the default slog handler. Logs go to stderr so command output
// on stdout stays pipeable; color follows the caller's TTY/NO_COLOR decision.
func Setup(level slog.Level, color bool) {
	tmpl, err := template.New("default").Parse(clogTemplate)
	if err != nil {
		panic(err)
	}

	h := clog.New(
		clog.WithWriter(os.Stderr),
		clog.WithColor(color),
		clog.WithLevel(level),
		clog.WithSource(true),
		clog.WithLevelFormatter(clogLevelFormatter),
		clog.WithTemplate(tmpl),
	)
	l := slog.New(h)
	slog.SetDefault(l)
}

// UseColorLog is the pre-convention entry point; new callers should pass their
// color decision through Setup.
func UseColorLog(level slog.Level) {
	Setup(level, true)
}
