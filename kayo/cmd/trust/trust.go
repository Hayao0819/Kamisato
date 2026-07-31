package trustcmd

import (
	"github.com/spf13/cobra"

	whitelistcmd "github.com/Hayao0819/Kamisato/kayo/cmd/trust/whitelist"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trust",
		Short: "Manage the local trust store (approved packages and maintainers)",
	}
	cmd.AddCommand(trustAddCmd(), trustListCmd(), trustRemoveCmd(), whitelistcmd.Cmd())
	return cmd
}
