package whitelistcmd

import (
	"github.com/spf13/cobra"

	addcmd "github.com/Hayao0819/Kamisato/kayo/cmd/trust/whitelist/add"
	removecmd "github.com/Hayao0819/Kamisato/kayo/cmd/trust/whitelist/remove"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whitelist",
		Short: "Manage the per-pkgbase auto-approve allowlist",
	}
	cmd.AddCommand(addcmd.Cmd(), removecmd.Cmd())
	return cmd
}
