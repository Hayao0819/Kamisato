package aurcmd

import (
	keygencmd "github.com/Hayao0819/Kamisato/ayato/cmd/aur/keygen"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aur",
		Short: "AUR catalog tooling",
	}
	cmd.AddCommand(keygencmd.Cmd())
	return cmd
}
