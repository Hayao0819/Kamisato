package cmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/version"
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "ayato",
		RunE: run,
	}
	cmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug mode")
	cmd.PersistentFlags().StringP("config", "c", "", "Config file")
	cliutil.SetVersion(cmd)
	cliutil.AddNoColorFlag(cmd)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.AddCommand(aurCmd())
	cmd.AddCommand(migrateCmd())
	cmd.AddCommand(kvCmd())
	cmd.AddCommand(repoCmd())
	cmd.AddCommand(version.Command())

	return cmd
}
