package whitelistcmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/kayo/cli"
	"github.com/Hayao0819/Kamisato/kayo/trust"
)

func addCmd() *cobra.Command {
	var note string
	cmd := &cobra.Command{
		Use:   "add <pkgname>",
		Short: "Unconditionally trust a pkgbase (skips review)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cli.LoadConfig(cmd)
			if err != nil {
				return err
			}
			store, err := trust.Open(cfg.ResolvedTrustStore())
			if err != nil {
				return err
			}
			store.AddWhitelist(args[0], note)
			return store.Save()
		},
	}
	cmd.Flags().StringVar(&note, "note", "", "why this pkgbase is whitelisted")
	return cmd
}
