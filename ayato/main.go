package main

import (
	"log/slog"
	"os"

	"github.com/Hayao0819/Kamisato/ayato/cmd"
)

func main() {
	if err := cmd.RootCmd().Execute(); err != nil {
		slog.Error("Failed to execute command", "error", err)
		os.Exit(1)
	}
}
