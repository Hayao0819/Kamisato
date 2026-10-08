package trustcmd

import (
	"github.com/spf13/cobra"

	addcmd "github.com/Hayao0819/Kamisato/kayo/cmd/trust/add"
	listcmd "github.com/Hayao0819/Kamisato/kayo/cmd/trust/list"
	removecmd "github.com/Hayao0819/Kamisato/kayo/cmd/trust/remove"
	whitelistcmd "github.com/Hayao0819/Kamisato/kayo/cmd/trust/whitelist"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trust",
		Short: "Manage the local trust store (approved packages and maintainers)",
	}
	cmd.AddCommand(addcmd.Cmd(), listcmd.Cmd(), removecmd.Cmd(), whitelistcmd.Cmd())
	return cmd
}
