package installcmd

import (
	_ "embed"

	"github.com/spf13/cobra"

	sharedhook "github.com/Hayao0819/Kamisato/internal/cli/hook"
	"github.com/Hayao0819/Kamisato/internal/pacman/hook"
)

//go:embed kayo-verify.hook.tmpl
var hookTemplate string

func Cmd() *cobra.Command {
	var configPath string
	return sharedhook.NewInstallCmd(sharedhook.InstallOptions{
		BinName:  "kayo",
		FileName: "kayo-verify.hook",
		Template: hookTemplate,
		SetupFlags: func(cmd *cobra.Command) {
			cmd.Flags().StringVar(&configPath, "config-path", "", "kayo config path to bake into the hook's Exec")
		},
		BuildExec: func(self, _ string) (string, error) {
			if err := hook.ValidateExecArg("--config-path", configPath); err != nil {
				return "", err
			}
			exec := self + " verify"
			if configPath != "" {
				exec += " -c " + configPath
			}
			return exec, nil
		},
	})
}
