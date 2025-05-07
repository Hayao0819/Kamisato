package cmd

import (
	"github.com/Hayao0819/ayaka/conf"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var subCmds = []*cobra.Command{}

func rootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "ayaka",
		Short: "Repository management tool",
		Long:  "Ayaka is a tool for managing your pacman repository",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return conf.LoadAppConfig(&conf.AppConfig)
		},
		SilenceUsage:  true,
	}
	cmd.CompletionOptions.HiddenDefaultCmd = true

	cmd.AddCommand(subCmds...)
	cmd.PersistentFlags().StringVarP(&conf.AppConfigPath, "config", "c", "", "config file path")
	cmd.PersistentFlags().StringVarP(&conf.AppConfig.RepoDir, "repodir", "r", "", "repository directory")
	viper.BindPFlag("repodir", cmd.PersistentFlags().Lookup("repodir"))

	return &cmd
}

func Execute() error {
	return rootCmd().Execute()
}
