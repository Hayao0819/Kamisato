// Package slogadapter adapts slog to printf-style logger interfaces used by KV
// backend clients.
package slogadapter

import (
	"fmt"
	"log/slog"
	"strings"
)

type Logger struct {
	logger *slog.Logger
}

func New(logger *slog.Logger) *Logger {
	if logger == nil {
		logger = slog.Default()
	}
	return &Logger{logger: logger}
}

func Default() *Logger {
	return New(slog.Default())
}

func message(format string, args ...interface{}) string {
	return strings.TrimSuffix(fmt.Sprintf(format, args...), "\n")
}

func (l *Logger) Printf(format string, args ...interface{}) {
	l.logger.Info(message(format, args...))
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	l.logger.Error(message(format, args...))
}

func (l *Logger) Warningf(format string, args ...interface{}) {
	l.logger.Warn(message(format, args...))
}

func (l *Logger) Infof(format string, args ...interface{}) {
	l.logger.Info(message(format, args...))
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	l.logger.Debug(message(format, args...))
}
