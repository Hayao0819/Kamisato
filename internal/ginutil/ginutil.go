// Package ginutil holds the bootstrap shared by the gin-based servers.
package ginutil

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/cliutil"
)

// Setup configures slog and gin's mode from the debug flag.
func Setup(cmd *cobra.Command, debug bool) {
	level := slog.LevelInfo
	mode := gin.ReleaseMode
	if debug {
		level = slog.LevelDebug
		mode = gin.DebugMode
	}
	cliutil.Setup(level, cliutil.ColorEnabled(cmd))
	gin.SetMode(mode)
}
