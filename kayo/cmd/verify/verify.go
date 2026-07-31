package verifycmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/pacman/hook"
	"github.com/Hayao0819/Kamisato/kayo/app"
	"github.com/Hayao0819/Kamisato/kayo/cli"
)

func Cmd() *cobra.Command {
	var strict bool
	cmd := &cobra.Command{
		Use:   "verify [pkgname...]",
		Short: "Check that packages being installed are trusted (pacman hook entry point)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cli.LoadConfig(cmd)
			if err != nil {
				return err
			}
			if len(args) == 0 {
				args = hook.StdinTargets()
			}
			return app.Verify(cmd.Context(), cfg, args, strict, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "fail the transaction even in warn mode")
	return cmd
}
