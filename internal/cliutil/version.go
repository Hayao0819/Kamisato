package cliutil

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/version"
)

func VersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build version information",
		Run: func(cmd *cobra.Command, _ []string) {
			v, c, d := version.BuildInfo()
			fmt.Fprintf(cmd.OutOrStdout(), "%s version %s\ncommit: %s\nbuilt:  %s\n", cmd.Root().Name(), v, c, d)
		},
	}
}
