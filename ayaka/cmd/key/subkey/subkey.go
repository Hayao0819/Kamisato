package subkeycmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cli"
	"github.com/Hayao0819/Kamisato/internal/pacman/sign"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subkey",
		Short: "Manage signing subkeys",
		Long:  "Add, revoke, or rotate the signing subkeys bound to the primary key. Rotating a subkey never changes the primary fingerprint, so downstream trust is preserved.",
	}
	cmd.AddCommand(addCmd(), revokeCmd(), rotateCmd())
	return cmd
}

func loadForMutation(cmd *cobra.Command) (*sign.SigningKey, string, error) {
	return cli.LoadSigningKey(cmd)
}
