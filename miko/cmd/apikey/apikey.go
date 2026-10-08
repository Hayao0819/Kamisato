package apikeycmd

import (
	generatecmd "github.com/Hayao0819/Kamisato/miko/cmd/apikey/generate"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apikey",
		Short: "Manage miko API keys",
	}
	cmd.AddCommand(generatecmd.Cmd())
	return cmd
}
