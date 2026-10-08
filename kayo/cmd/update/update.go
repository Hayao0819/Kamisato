package updatecmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/kayo/cmd/internal/settings"
	"github.com/Hayao0819/Kamisato/kayo/config"
	"github.com/Hayao0819/Kamisato/kayo/service"
)

type updateFunc func(context.Context, *config.KayoConfig, string, bool, bool) (*service.UpdateResult, error)

func Cmd() *cobra.Command {
	return newCommand(service.Update)
}

func newCommand(update updateFunc) *cobra.Command {
	var approve, force bool
	cmd := &cobra.Command{
		Use:   "update <package|dir|git-url>",
		Short: "Review changes since the approved commit and re-pin with --approve",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := settings.Load(cmd.Flags())
			if err != nil {
				return err
			}
			result, err := update(cmd.Context(), cfg, args[0], approve, force)
			if result != nil {
				printUpdate(cmd.OutOrStdout(), result)
			}
			return err
		},
	}
	cmd.Flags().BoolVar(&approve, "approve", false, "advance the pin to the current commit/maintainer")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "approve despite high-severity findings")
	return cmd
}
