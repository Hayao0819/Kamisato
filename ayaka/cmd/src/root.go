// Package srccmd groups the commands that read or edit the source repository
// working tree (.ayakarc and the PKGBUILD dirs it declares).
package srccmd

import (
	"github.com/spf13/cobra"

	aurcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/src/aur"
	bumpcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/src/bump"
	listcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/src/list"
	srcinfocmd "github.com/Hayao0819/Kamisato/ayaka/cmd/src/srcinfo"
	statuscmd "github.com/Hayao0819/Kamisato/ayaka/cmd/src/status"
	submodulescmd "github.com/Hayao0819/Kamisato/ayaka/cmd/src/submodules"
)

// Cmd builds the `ayaka src` command group.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "src",
		Short: "Inspect and maintain the source repository working tree",
	}
	cmd.AddCommand(
		listcmd.Cmd(),
		statuscmd.Cmd(),
		srcinfocmd.Cmd(),
		bumpcmd.Cmd(),
		aurcmd.Cmd(),
		submodulescmd.Cmd(),
	)
	return cmd
}
