package updatecmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/kayo/app"
	"github.com/Hayao0819/Kamisato/kayo/cli"
)

func Cmd() *cobra.Command {
	var approve, force bool
	cmd := &cobra.Command{
		Use:   "update <package|git-url>",
		Short: "Review changes since the approved commit and re-pin with --approve",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cli.LoadConfig(cmd)
			if err != nil {
				return err
			}
			result, err := app.Update(cmd.Context(), cfg, args[0], approve, force)
			if result != nil {
				cli.PrintUpdate(cmd.OutOrStdout(), result)
			}
			return err
		},
	}
	cmd.Flags().BoolVar(&approve, "approve", false, "advance the pin to the current commit/maintainer")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "approve despite high-severity findings")
	return cmd
}
