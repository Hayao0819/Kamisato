package cmd

import (
	"log/slog"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/nahi/cobrautils"
	"github.com/spf13/cobra"
)

var subCmds = cobrautils.Registory{}

func RootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "ayaka",
		Short: "Repository management tool",
		Long:  "Ayaka is a tool for managing pacman repositories.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			c, err := conf.LoadAyakaConfig(cmd.Flags())
			if err != nil {
				return err
			}
			config = c

			if config.Debug {
				utils.UseColorLog(slog.LevelDebug)
			} else {
				utils.UseColorLog(slog.LevelInfo)
			}

			if config.LegacyRepoDir != "" || config.LegacyDestDir != "" {
				slog.Warn("Using legacy configuration fields 'repodir' or 'destdir' is deprecated. Please migrate to the new 'repos' field.")
				config.Repos = append(config.Repos, struct {
					Dir     string `koanf:"dir" json:"dir"`
					DestDir string `koanf:"destdir" json:"destdir"`
				}{
					Dir:     config.LegacyRepoDir,
					DestDir: config.LegacyDestDir,
				})
			}
			if err := initSrcRepos(); err != nil {
				return err
			}

			return nil
		},
		SilenceUsage: true,
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: true,
		},
		SilenceErrors: true,
	}

	subCmds.Bind(&cmd)
	cmd.PersistentFlags().StringP("repodir", "", "", "Repository directory")
	cmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug mode")

	return &cmd
}
