package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/internal/gitcmd"
	"github.com/Hayao0819/Kamisato/kayo/audit"
	"github.com/Hayao0819/Kamisato/kayo/cmd/shared"
	"github.com/Hayao0819/Kamisato/kayo/gitserve"
	"github.com/Hayao0819/Kamisato/kayo/trust"
	"github.com/spf13/cobra"
)

func updateCmd() *cobra.Command {
	var approve, force bool
	cmd := &cobra.Command{
		Use:   "update <package|git-url>",
		Short: "Review changes since the approved commit and re-pin with --approve",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := shared.LoadConfig(cmd)
			if err != nil {
				return err
			}

			r, cleanup, err := shared.Resolve(cmd.Context(), cfg, args[0], "")
			defer cleanup()
			if err != nil {
				return err
			}

			store, err := trust.Open(cfg.ResolvedTrustStore())
			if err != nil {
				return err
			}
			ap, ok := store.Approval(r.Pkgbase)
			if !ok {
				return errwrap.NewErrf("%s is not tracked; use 'kayo trust add' first", r.Pkgbase)
			}

			report, err := audit.Scan(r.Dir)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "package: %s (source %s)\n", r.Pkgbase, r.Source)
			if ap.Maintainer != r.Maintainer {
				fmt.Fprintf(out, "MAINTAINER CHANGED: %q -> %q\n", ap.Maintainer, r.Maintainer)
			}
			switch {
			case ap.Commit == r.Commit:
				fmt.Fprintln(out, "no commit change since approval")
			default:
				fmt.Fprintf(out, "commit: %s -> %s\n", shared.Short(ap.Commit), shared.Short(r.Commit))
				if names := diffNames(cmd.Context(), r.Dir, ap.Commit, r.Commit); len(names) > 0 {
					fmt.Fprintln(out, "changed files:")
					for _, n := range names {
						fmt.Fprintf(out, "  %s\n", n)
					}
				}
			}
			shared.PrintFindings(out, report)

			if !approve {
				fmt.Fprintln(out, "(dry run; re-run with --approve to advance the pin)")
				return nil
			}
			if report.Max() >= audit.SevHigh && !force {
				return errwrap.NewErr("refusing to approve: high-severity findings (use --force)")
			}

			if err := r.RequirePinnedCommit(); err != nil {
				return err
			}
			if err := gitserve.Materialize(cmd.Context(), cfg.ServedRoot(), r.Pkgbase, r.Dir, r.Commit); err != nil {
				return errwrap.WrapErr(err, "failed to re-pin reviewed commit")
			}
			store.Approve(trust.Approval{
				Pkgbase:    r.Pkgbase,
				Source:     r.Source,
				Maintainer: r.Maintainer,
				Commit:     r.Commit,
				AuditMax:   report.Max().String(),
			})
			store.TrustMaintainer(r.Source, r.Maintainer, "via update "+args[0])
			if err := store.Save(); err != nil {
				return err
			}
			fmt.Fprintf(out, "re-pinned %s at %s\n", r.Pkgbase, shared.Short(r.Commit))
			return nil
		},
	}
	cmd.Flags().BoolVar(&approve, "approve", false, "advance the pin to the current commit/maintainer")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "approve despite high-severity findings")
	return cmd
}

// diffNames is best-effort: nil on any git error, e.g. a force-pushed history
// where the old commit is gone.
func diffNames(ctx context.Context, dir, from, to string) []string {
	if from == "" || to == "" {
		return nil
	}
	out, err := gitcmd.Output(ctx, dir, "diff", "--name-only", from, to)
	if err != nil {
		return nil
	}
	var names []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			names = append(names, line)
		}
	}
	return names
}
