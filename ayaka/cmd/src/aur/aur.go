package aurcmd

import (
	addcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/src/aur/add"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	command := &cobra.Command{Use: "aur", Short: "Manage PKGBUILDs taken from the AUR"}
	command.AddCommand(addcmd.Cmd())
	return command
}
