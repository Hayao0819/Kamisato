package logger

import (
	"fmt"
	"log/slog"
	"strings"
)

// Implements: https://pkg.go.dev/github.com/dgraph-io/badger/v4#Logger
type CloudflareSlog struct {
	logger *slog.Logger
}

func (c *CloudflareSlog) str(format string, args ...interface{}) string {
	s := fmt.Sprintf(format, args...)
	s = strings.TrimSuffix(s, "\n")
	return s
}

func (c *CloudflareSlog) Printf(format string, args ...interface{}) {
	s := c.str(format, args...)
	c.logger.Info(s)
}

func (c *CloudflareSlog) Errorf(format string, args ...interface{}) {
	s := c.str(format, args...)
	c.logger.Error(s)
}

func (c *CloudflareSlog) Warningf(format string, args ...interface{}) {
	s := c.str(format, args...)
	c.logger.Warn(s)
}

func (c *CloudflareSlog) Infof(format string, args ...interface{}) {
	s := c.str(format, args...)
	c.logger.Info(s)
}

func (b *CloudflareSlog) Debugf(format string, args ...interface{}) {
	s := b.str(format, args...)
	b.logger.Debug(s)
}

func NewCloudflareSlog(logger *slog.Logger) *CloudflareSlog {
	return &CloudflareSlog{
		logger: logger,
	}
}

func Default() *CloudflareSlog {
	return NewCloudflareSlog(slog.Default())
}
