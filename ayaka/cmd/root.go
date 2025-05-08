package cmd

import (
	"github.com/Hayao0819/Kamisato/conf"
	"github.com/Hayao0819/nahi/cobrautils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var subCmds = cobrautils.Registory{}
var config *conf.AyakaConfig

func rootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "ayaka",
		Short: "Repository management tool",
		Long:  "Ayaka is a tool for managing your pacman repository",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			c, err := conf.LoadAyakaConfig()
			if err != nil {
				return err
			}
			config = c
			return nil
		},
		SilenceUsage: true,
	}
	cmd.CompletionOptions.HiddenDefaultCmd = true

	subCmds.Bind(&cmd)
	// cmd.PersistentFlags().StringVarP(&conf.AppConfigPath, "config", "c", "", "config file path")
	// cmd.PersistentFlags().StringVarP(&conf.AppConfig.RepoDir, "repodir", "r", "", "repository directory")
	viper.BindPFlag("repodir", cmd.PersistentFlags().Lookup("repodir"))

	return &cmd
}

func Execute() error {
	return rootCmd().Execute()
}
