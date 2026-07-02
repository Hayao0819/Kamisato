package cmd

import (
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/kayo/cmd/shared"
	"github.com/Hayao0819/Kamisato/kayo/trust"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
	"github.com/Hayao0819/Kamisato/pkg/pacman/alpm"
	"github.com/Hayao0819/Kamisato/pkg/pacman/hook"
	"github.com/spf13/cobra"
)

// verifyTarget is a resolved install target plus the delegation flag the hook
// needs to gate it.
type verifyTarget struct {
	pkg               aurweb.Pkg
	source            string
	delegatedVerified bool
}

// reportTrust writes one status line per target (in the order given) and reports
// whether any target needs review. It routes every target through the canonical
// trust.EvaluateResolved, the same gate the federation merge uses, so the
// install-time verdict cannot diverge from the resolve-time one.
func reportTrust(w io.Writer, store *trust.Store, order []string, found map[string]verifyTarget) (needsReview bool) {
	for _, name := range order {
		t, ok := found[name]
		if !ok {
			continue // official-repo package, or unknown to the AUR
		}
		v := store.EvaluateResolved(t.source, t.pkg.PackageBase, t.pkg.Maintainer, t.delegatedVerified)
		if v.Decision == trust.Trusted {
			fmt.Fprintf(w, "  ok      %s (%s)\n", name, t.source)
			continue
		}
		fmt.Fprintf(w, "  REVIEW  %s (%s) — %s\n", name, t.source, strings.Join(v.Reasons, "; "))
		needsReview = true
	}
	return needsReview
}

// verifyCmd is the install-time backstop the pacman PreTransaction hook calls.
// Official-repo packages are not ours to gate, so they are skipped.
func verifyCmd() *cobra.Command {
	var strict bool
	cmd := &cobra.Command{
		Use:   "verify [pkgname...]",
		Short: "Check that packages being installed are trusted (pacman hook entry point)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := shared.LoadConfig(cmd)
			if err != nil {
				return err
			}
			names := args
			if len(names) == 0 {
				names = hook.StdinTargets() // pacman NeedsTargets passes targets on stdin
			}
			if len(names) == 0 {
				return nil
			}

			store, err := trust.Open(cfg.ResolvedTrustStore())
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			comp, err := shared.BuildComposite(ctx, cfg)
			if err != nil {
				return err
			}
			up := shared.UpstreamClient(cfg)
			enforce := strict || cfg.ResolvedEnforceMode() == "enforce"

			found := make(map[string]verifyTarget, len(names))
			var unknown []string
			for _, name := range names {
				if pkg, source, delegatedVerified, ok := comp.Resolve(ctx, name); ok {
					found[name] = verifyTarget{pkg, source, delegatedVerified}
				} else {
					unknown = append(unknown, name)
				}
			}

			// Of the locally-unknown targets, official-repo packages (present in a
			// sync DB) are not ours to gate; only the foreign (AUR/local) ones need
			// an upstream lookup. Resolve those in ONE batched call, not one per
			// package — a -Syu otherwise floods the AUR with a request per target.
			if len(unknown) > 0 && up != nil {
				foreign := unknown
				if sync, serr := alpm.SyncPackages(); serr == nil {
					foreign = foreign[:0]
					for _, n := range unknown {
						if !sync[n] {
							foreign = append(foreign, n)
						}
					}
				} else {
					slog.Warn("could not list sync-repo packages; treating all unknown targets as foreign", "error", serr)
				}
				if len(foreign) > 0 {
					ps, ierr := up.Info(ctx, foreign)
					if ierr != nil {
						// Fail closed: a lookup we cannot complete must never silently
						// pass an AUR package that might need review.
						if enforce {
							return utils.WrapErr(ierr, "could not verify AUR packages upstream (failing closed)")
						}
						slog.Warn("upstream lookup failed; AUR packages unverified this run", "error", ierr)
					}
					for _, p := range ps {
						found[p.Name] = verifyTarget{pkg: p, source: "aur"}
					}
				}
			}

			if reportTrust(cmd.OutOrStdout(), store, names, found) && enforce {
				return utils.NewErr("untrusted packages in transaction; review with 'kayo update' or 'kayo trust add'")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "fail the transaction even in warn mode")
	return cmd
}
