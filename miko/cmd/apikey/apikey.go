package apikeycmd

import "github.com/spf13/cobra"

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apikey",
		Short: "Manage miko API keys",
	}
	cmd.AddCommand(generateCmd())
	return cmd
}
