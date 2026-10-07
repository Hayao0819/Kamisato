// Package server provides common Gin server startup and shutdown behavior.
package server

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
)

// Setup configures slog and gin's mode from the debug flag.
func Setup(cmd *cobra.Command, debug bool) {
	level := slog.LevelInfo
	mode := gin.ReleaseMode
	if debug {
		level = slog.LevelDebug
		mode = gin.DebugMode
	}
	cmdline.Setup(level, cmdline.ColorEnabled(cmd))
	gin.SetMode(mode)
}
