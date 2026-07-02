package hookcmd

import (
	_ "embed"

	sharedhook "github.com/Hayao0819/Kamisato/internal/hookcmd"
	"github.com/spf13/cobra"
)

//go:embed ayaka-upload.hook.tmpl
var uploadHookTemplate string

const uploadHookFileName = "ayaka-upload.hook"

// Cmd manages the pacman PostTransaction hook for build-once-share-many: a
// locally-built package lands in the ayato repo so other machines pull the binary.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Manage the pacman hook that uploads installed packages to ayato",
	}
	cmd.AddCommand(hookInstallCmd(), sharedhook.NewUninstallCmd(uploadHookFileName), hookUploadCmd())
	return cmd
}
