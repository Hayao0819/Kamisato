package addcmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/kayo/cmd/internal/settings"
	"github.com/Hayao0819/Kamisato/kayo/trust"
)

func Cmd() *cobra.Command {
	var note string
	cmd := &cobra.Command{
		Use:   "add <pkgbase>",
		Short: "Unconditionally trust a pkgbase (skips review)",
		Long: "Bypass package review and maintainer-change checks for this pkgbase.\n" +
			"This does not audit the recipe or pin a reviewed commit; use 'trust add' for that.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := settings.Load(cmd.Flags())
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
