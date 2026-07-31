// Package srccmd groups the commands that read or edit the source repository
// working tree (.ayakarc and the PKGBUILD dirs it declares).
package srccmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	aurcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/src/aur"
	bumpcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/src/bump"
	listcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/src/list"
	nvbumpcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/src/nvbump"
	pullcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/src/pull"
	srcinfocmd "github.com/Hayao0819/Kamisato/ayaka/cmd/src/srcinfo"
	statuscmd "github.com/Hayao0819/Kamisato/ayaka/cmd/src/status"
	submodulescmd "github.com/Hayao0819/Kamisato/ayaka/cmd/src/submodules"
)

// Cmd builds the `ayaka src` command group.
func Cmd(runtime *app.Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "src",
		Short: "Inspect and maintain the source repository working tree",
	}
	cmd.AddCommand(
		listcmd.Cmd(runtime),
		statuscmd.Cmd(runtime),
		srcinfocmd.Cmd(runtime),
		bumpcmd.Cmd(runtime),
		nvbumpcmd.Cmd(runtime),
		pullcmd.Cmd(runtime),
		aurcmd.Cmd(runtime),
		submodulescmd.Cmd(runtime),
	)
	return cmd
}
