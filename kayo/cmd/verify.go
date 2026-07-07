package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/kayo/clonecache"
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

// reportTrust writes a status line per target and reports whether any needs
// review, routing each through the canonical trust.EvaluateResolved — the same
// gate the federation merge uses — so the install-time verdict cannot diverge
// from the resolve-time one.
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

// reportCloneDrift reports whether any target's clone-cache checkout drifted off
// its approved commit; targets with no pin or not yet cloned are skipped, and a
// git error is logged not fatal — a build backstop must not fail on an unreadable
// local repo.
func reportCloneDrift(ctx context.Context, w io.Writer, store *trust.Store, root string, order []string, found map[string]verifyTarget) (drifted bool) {
	for _, name := range order {
		t, ok := found[name]
		if !ok {
			continue
		}
		base := t.pkg.PackageBase
		ap, ok := store.Approval(base)
		if !ok || ap.Commit == "" {
			continue
		}
		res, err := clonecache.Check(ctx, root, base, ap.Commit)
		if err != nil {
			slog.Warn("could not read clone cache; skipping pin check", "pkgbase", base, "error", err)
			continue
		}
		if res.Drifted() {
			fmt.Fprintf(w, "  DRIFT   %s (%s) — clone cache at %s, approved %s\n", name, base, shared.Short(res.Head), shared.Short(ap.Commit))
			drifted = true
		}
	}
	return drifted
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
			comp, _, err := shared.BuildComposite(ctx, cfg)
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
							return errwrap.WrapErr(ierr, "could not verify AUR packages upstream (failing closed)")
						}
						slog.Warn("upstream lookup failed; AUR packages unverified this run", "error", ierr)
					}
					for _, p := range ps {
						found[p.Name] = verifyTarget{pkg: p, source: "aur"}
					}
				}
			}

			out := cmd.OutOrStdout()
			needsReview := reportTrust(out, store, names, found)
			// A pinned package whose local clone drifted off the approved commit is
			// as much a build-time concern as an untrusted maintainer, so it gates
			// the transaction the same way.
			if reportCloneDrift(ctx, out, store, cfg.ResolvedYayCacheDir(), names, found) {
				needsReview = true
			}
			if needsReview && enforce {
				return errwrap.NewErr("untrusted packages in transaction; review with 'kayo update' or 'kayo trust add'")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "fail the transaction even in warn mode")
	return cmd
}
