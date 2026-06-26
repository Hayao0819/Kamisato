package cmd

import (
	"fmt"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/sara/audit"
	"github.com/Hayao0819/Kamisato/sara/gitserve"
	"github.com/Hayao0819/Kamisato/sara/trust"
	"github.com/spf13/cobra"
)

func trustCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trust",
		Short: "Manage the local trust store (approved packages and maintainers)",
	}
	cmd.AddCommand(trustAddCmd(), trustListCmd(), trustRemoveCmd())
	return cmd
}

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
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}

			r, cleanup, err := resolve(cmd.Context(), cfg, args[0], ref)
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
			printReport(out, r, report, store.Evaluate(r.Source, r.Pkgbase, r.Maintainer))
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
			fmt.Fprintf(out, "trusted and pinned %s at %s (maintainer %q)\n", r.Pkgbase, short(r.Commit), r.Maintainer)
			return nil
		},
	}
	cmd.Flags().StringVar(&ref, "ref", "", "git ref or commit to pin")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "trust despite high-severity audit findings")
	return cmd
}

func trustListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List trusted maintainers and approved packages",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			store, err := trust.Open(cfg.ResolvedTrustStore())
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "maintainers:")
			for _, m := range store.Maintainers() {
				fmt.Fprintf(out, "  %s/%s\n", m.Source, m.Account)
			}
			fmt.Fprintln(out, "packages:")
			for _, a := range store.Approvals() {
				fmt.Fprintf(out, "  %s @ %s (maintainer %q, source %s)\n", a.Pkgbase, short(a.Commit), a.Maintainer, a.Source)
			}
			return nil
		},
	}
}

func trustRemoveCmd() *cobra.Command {
	var maintainer string
	cmd := &cobra.Command{
		Use:   "rm [pkgbase]",
		Short: "Remove a package approval, or a maintainer with --maintainer source/account",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			store, err := trust.Open(cfg.ResolvedTrustStore())
			if err != nil {
				return err
			}

			switch {
			case maintainer != "":
				source, account, ok := strings.Cut(maintainer, "/")
				if !ok {
					return utils.NewErr("--maintainer must be source/account, e.g. aur/jguer")
				}
				store.UntrustMaintainer(source, account)
			case len(args) == 1:
				store.RemoveApproval(args[0])
				_ = gitserve.Remove(cfg.ServedRoot(), args[0])
			default:
				return utils.NewErr("specify a pkgbase or --maintainer source/account")
			}
			return store.Save()
		},
	}
	cmd.Flags().StringVar(&maintainer, "maintainer", "", "remove a trusted maintainer (source/account) instead of a package")
	return cmd
}

func short(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}
