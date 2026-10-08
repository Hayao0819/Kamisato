package uninstallcmd

import (
	"github.com/spf13/cobra"

	sharedhook "github.com/Hayao0819/Kamisato/internal/cli/hook"
)

func Cmd() *cobra.Command {
	return sharedhook.NewUninstallCmd("kayo-verify.hook")
}
