package subkeycmd

import (
	addcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/key/subkey/add"
	revokecmd "github.com/Hayao0819/Kamisato/ayaka/cmd/key/subkey/revoke"
	rotatecmd "github.com/Hayao0819/Kamisato/ayaka/cmd/key/subkey/rotate"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subkey",
		Short: "Manage signing subkeys",
		Long:  "Add, revoke, or rotate the signing subkeys bound to the primary key. Rotating a subkey never changes the primary fingerprint, so downstream trust is preserved.",
	}
	cmd.AddCommand(addcmd.Cmd(), revokecmd.Cmd(), rotatecmd.Cmd())
	return cmd
}
