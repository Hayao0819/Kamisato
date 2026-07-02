package admincmd

import (
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Manage ayato admin allowlist",
		Long:  "List, add, and remove ayato admins. Requires a logged-in server with a CLI token.",
	}
	shared.AddServerFlag(cmd)

	cmd.AddCommand(adminListCmd(), adminAddCmd(), adminRemoveCmd())
	return cmd
}
