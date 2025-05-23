package logger

import (
	"fmt"
	"log/slog"
	"strings"
)

// Implements: https://pkg.go.dev/github.com/dgraph-io/badger/v4#Logger
type BadgerSlog struct {
	logger *slog.Logger
}

func (b *BadgerSlog) str(format string, args ...interface{}) string {
	s := fmt.Sprintf(format, args...)
	s = strings.TrimSuffix(s, "\n")
	return s
}

func (b *BadgerSlog) Errorf(format string, args ...interface{}) {
	s := b.str(format, args...)
	b.logger.Error(s)
}

func (b *BadgerSlog) Warningf(format string, args ...interface{}) {
	s := b.str(format, args...)
	b.logger.Warn(s)
}

func (b *BadgerSlog) Infof(format string, args ...interface{}) {
	s := b.str(format, args...)
	b.logger.Info(s)
}

func (b *BadgerSlog) Debugf(format string, args ...interface{}) {
	s := b.str(format, args...)
	b.logger.Debug(s)
}

func NewBadgerSlog(logger *slog.Logger) *BadgerSlog {
	return &BadgerSlog{
		logger: logger,
	}
}

func Default() *BadgerSlog {
	return NewBadgerSlog(slog.Default())
}
