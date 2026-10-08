// Package server provides common Gin server startup and shutdown behavior.
package server

import "github.com/gin-gonic/gin"

// SetMode configures Gin's mode. Logging and command-line flags are configured
// separately by the process entry point.
func SetMode(debug bool) {
	mode := gin.ReleaseMode
	if debug {
		mode = gin.DebugMode
	}
	gin.SetMode(mode)
}
