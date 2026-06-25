// Package logger adapts slog to the cloudflare-go client's logger interface.
package logger

import (
	"fmt"
	"log/slog"
	"strings"
)

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

func (c *CloudflareSlog) Debugf(format string, args ...interface{}) {
	s := c.str(format, args...)
	c.logger.Debug(s)
}

func NewCloudflareSlog(logger *slog.Logger) *CloudflareSlog {
	return &CloudflareSlog{
		logger: logger,
	}
}

func Default() *CloudflareSlog {
	return NewCloudflareSlog(slog.Default())
}
