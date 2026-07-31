package hookcmd

import (
	_ "embed"

	"github.com/spf13/cobra"

	sharedhook "github.com/Hayao0819/Kamisato/internal/hookcmd"
)

//go:embed kayo-verify.hook.tmpl
var hookTemplate string

const hookFileName = "kayo-verify.hook"

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Manage the pacman PreTransaction hook that runs 'kayo verify'",
	}
	cmd.AddCommand(hookInstallCmd(), sharedhook.NewUninstallCmd(hookFileName))
	return cmd
}
