package admincmd

import (
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/remote"
	addcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/server/admin/add"
	listcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/server/admin/list"
	removecmd "github.com/Hayao0819/Kamisato/ayaka/cmd/server/admin/remove"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Manage ayato admin allowlist",
		Long:  "List, add, and remove ayato admins. Requires a logged-in server with a CLI token.",
	}
	remote.AddPersistentServerFlag(cmd)

	cmd.AddCommand(listcmd.Cmd(), addcmd.Cmd(), removecmd.Cmd())
	return cmd
}
