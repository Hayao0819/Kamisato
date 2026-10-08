package uninstallcmd

import (
	sharedhook "github.com/Hayao0819/Kamisato/internal/cli/hook"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command { return sharedhook.NewUninstallCmd("ayaka-upload.hook") }
