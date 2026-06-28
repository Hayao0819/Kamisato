package trustcmd

import (
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trust",
		Short: "Manage the local trust store (approved packages and maintainers)",
	}
	cmd.AddCommand(trustAddCmd(), trustListCmd(), trustRemoveCmd())
	return cmd
}
