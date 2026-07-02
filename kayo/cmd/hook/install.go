package hookcmd

import (
	sharedhook "github.com/Hayao0819/Kamisato/internal/hookcmd"
	"github.com/Hayao0819/Kamisato/pkg/pacman/hook"
	"github.com/spf13/cobra"
)

func hookInstallCmd() *cobra.Command {
	var configPath string
	return sharedhook.NewInstallCmd(sharedhook.InstallOptions{
		BinName:  "kayo",
		FileName: hookFileName,
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
