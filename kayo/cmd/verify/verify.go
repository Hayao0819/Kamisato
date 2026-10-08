package verifycmd

import (
	"context"

	"github.com/spf13/cobra"

	sharedhook "github.com/Hayao0819/Kamisato/internal/cli/hook"
	"github.com/Hayao0819/Kamisato/kayo/cmd/internal/settings"
	kayoconfig "github.com/Hayao0819/Kamisato/kayo/config"
	"github.com/Hayao0819/Kamisato/kayo/service"
)

func Cmd() *cobra.Command {
	return newCommand(service.Verify)
}

func newCommand(verify func(context.Context, *kayoconfig.KayoConfig, []string, bool) ([]service.Verification, error)) *cobra.Command {
	var strict bool
	cmd := &cobra.Command{
		Use:   "verify [pkgname...]",
		Short: "Check that packages being installed are trusted (pacman hook entry point)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := settings.Load(cmd.Flags())
			if err != nil {
				return err
			}
			if len(args) == 0 {
				args, err = sharedhook.ReadTargets(cmd.InOrStdin())
				if err != nil {
					return err
				}
			}
			results, err := verify(cmd.Context(), cfg, args, strict)
			printResults(cmd.OutOrStdout(), results)
			return err
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "fail the transaction even in warn mode")
	return cmd
}
