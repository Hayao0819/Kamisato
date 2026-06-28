package hookcmd

import (
	_ "embed"

	"github.com/spf13/cobra"
)

//go:embed ayaka-upload.hook.tmpl
var uploadHookTemplate string

const uploadHookFileName = "ayaka-upload.hook"

// Cmd manages the pacman PostTransaction hook that publishes every freshly
// installed package to an ayato repository. The build-once-share-many flow: a
// package built locally lands in the repo so other machines pull it as a binary.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Manage the pacman hook that uploads installed packages to ayato",
	}
	cmd.AddCommand(hookInstallCmd(), hookUninstallCmd(), hookUploadCmd())
	return cmd
}
