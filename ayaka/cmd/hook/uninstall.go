package hookcmd

import (
	"github.com/Hayao0819/Kamisato/pkg/pacman/hook"
	"github.com/spf13/cobra"
)

func hookUninstallCmd() *cobra.Command {
	return hook.UninstallCmd(uploadHookFileName)
}
