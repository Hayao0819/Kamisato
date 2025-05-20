package cmd

import (
	"log/slog"

	"github.com/Hayao0819/Kamisato/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/nahi/cobrautils"
	"github.com/spf13/cobra"
)

var subCmds = cobrautils.Registory{}
var config *conf.AyakaConfig

func RootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "ayaka",
		Short: "Repository management tool",
		Long:  "Ayaka is a tool for managing your pacman repository",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			c, err := conf.LoadAyakaConfig(cmd.Flags())
			if err != nil {
				return err
			}
			config = c

			if c.Debug {
				// println("debug mode")
				utils.UseColorLog(slog.LevelDebug)
			} else {
				utils.UseColorLog(slog.LevelInfo)
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
	// cmd.PersistentFlags().StringVarP(&config., "config", "c", "", "config file path")
	cmd.PersistentFlags().StringP("repodir", "r", "", "repository directory")
	cmd.PersistentFlags().BoolP("debug", "d", false, "enable debug mode")

	// TODO: Implement it with koanf
	// viper.BindPFlag("repodir", cmd.PersistentFlags().Lookup("repodir"))

	return &cmd
}
