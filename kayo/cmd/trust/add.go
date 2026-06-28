package trustcmd

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/kayo/audit"
	"github.com/Hayao0819/Kamisato/kayo/cmd/shared"
	"github.com/Hayao0819/Kamisato/kayo/gitserve"
	"github.com/Hayao0819/Kamisato/kayo/trust"
	"github.com/spf13/cobra"
)

// trustAddCmd whitelists a package: it resolves the source, audits the recipe,
// pins the current commit, and trusts the package's maintainer ACCOUNT (from the
// AUR RPC, not any git email). It refuses on high-severity findings unless
// --force.
func trustAddCmd() *cobra.Command {
	var ref string
	var force bool
	cmd := &cobra.Command{
		Use:   "add <package|git-url>",
		Short: "Whitelist a package and trust its current maintainer account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := shared.LoadConfig(cmd)
			if err != nil {
				return err
			}

			r, cleanup, err := shared.Resolve(cmd.Context(), cfg, args[0], ref)
			defer cleanup()
			if err != nil {
				return err
			}

			report, err := audit.Scan(r.Dir)
			if err != nil {
				return err
			}
			store, err := trust.Open(cfg.ResolvedTrustStore())
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			shared.PrintReport(out, r, report, store.Evaluate(r.Source, r.Pkgbase, r.Maintainer))
			shared.PrintLLMAdvisory(cmd.Context(), out, cfg, r.Dir, false)
			if report.Max() >= audit.SevHigh && !force {
				return utils.NewErr("refusing to trust: high-severity findings (use --force to override)")
			}

			// Pin the reviewed commit by serving it ourselves (variant B), so a
			// later clone gets exactly what was audited.
			if err := gitserve.Materialize(cmd.Context(), cfg.ServedRoot(), r.Pkgbase, r.Dir, r.Commit); err != nil {
				return utils.WrapErr(err, "failed to pin reviewed commit")
			}

			store.Approve(trust.Approval{
				Pkgbase:    r.Pkgbase,
				Source:     r.Source,
				Maintainer: r.Maintainer,
				Commit:     r.Commit,
				AuditMax:   report.Max().String(),
			})
			store.TrustMaintainer(r.Source, r.Maintainer, "via "+args[0])
			if err := store.Save(); err != nil {
				return err
			}
			fmt.Fprintf(out, "trusted and pinned %s at %s (maintainer %q)\n", r.Pkgbase, shared.Short(r.Commit), r.Maintainer)
			return nil
		},
	}
	cmd.Flags().StringVar(&ref, "ref", "", "git ref or commit to pin")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "trust despite high-severity audit findings")
	return cmd
}
