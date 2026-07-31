package whitelistcmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/kayo/cli"
	"github.com/Hayao0819/Kamisato/kayo/trust"
)

func removeCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <pkgname>",
		Aliases: []string{"rm"},
		Short:   "Remove a pkgbase from the allowlist",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cli.LoadConfig(cmd)
			if err != nil {
				return err
			}
			store, err := trust.Open(cfg.ResolvedTrustStore())
			if err != nil {
				return err
			}
			store.RemoveWhitelist(args[0])
			return store.Save()
		},
	}
}
