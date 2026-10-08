// Package settings decodes shared configuration input at command execution.
package settings

import (
	"log/slog"

	"github.com/Hayao0819/Kamisato/ayaka/config"
	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/spf13/cobra"
)

func Load(command *cobra.Command) (*config.AyakaConfig, error) {
	path, err := command.Flags().GetString("config")
	if err != nil {
		return nil, err
	}
	cfg, err := config.LoadAyakaConfig(command.Flags(), path)
	if err != nil {
		return nil, err
	}
	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}
	cmdline.Setup(level, cmdline.ColorEnabled(command))
	return cfg, nil
}
