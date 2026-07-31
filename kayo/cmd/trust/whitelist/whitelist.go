package whitelistcmd

import "github.com/spf13/cobra"

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whitelist",
		Short: "Manage the per-pkgbase auto-approve allowlist",
	}
	cmd.AddCommand(addCmd(), removeCmd())
	return cmd
}
