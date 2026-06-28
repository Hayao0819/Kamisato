package hookcmd

import (
	_ "embed"

	"github.com/spf13/cobra"
)

//go:embed kayo-verify.hook.tmpl
var hookTemplate string

const hookFileName = "kayo-verify.hook"

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Manage the pacman PreTransaction hook that runs 'kayo verify'",
	}
	cmd.AddCommand(hookInstallCmd(), hookUninstallCmd())
	return cmd
}
