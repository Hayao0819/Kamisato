package trustcmd

import (
	"github.com/Hayao0819/Kamisato/kayo/cmd/shared"
	"github.com/Hayao0819/Kamisato/kayo/trust"
	"github.com/spf13/cobra"
)

// whitelistCmd manages the unconditional per-pkgbase allowlist: a whitelisted
// pkgbase evaluates as trusted without a reviewed pin, bypassing the new-package
// and maintainer-change checks. It is blunter than `trust add` (which reviews and
// pins a commit); use it only for packages you never want gated.
func whitelistCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whitelist",
		Short: "Manage the per-pkgbase auto-approve allowlist",
	}
	cmd.AddCommand(whitelistAddCmd(), whitelistRemoveCmd())
	return cmd
}

func whitelistAddCmd() *cobra.Command {
	var note string
	cmd := &cobra.Command{
		Use:   "add <pkgbase>",
		Short: "Unconditionally trust a pkgbase (skips review)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := shared.LoadConfig(cmd)
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

func whitelistRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <pkgbase>",
		Short: "Remove a pkgbase from the allowlist",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := shared.LoadConfig(cmd)
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
