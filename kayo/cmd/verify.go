package cmd

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/kayo/cmd/shared"
	"github.com/Hayao0819/Kamisato/kayo/trust"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
	"github.com/Hayao0819/Kamisato/pkg/pacman/hook"
	"github.com/spf13/cobra"
)

// verifyCmd is the install-time backstop the pacman PreTransaction hook calls.
// It resolves each target to its source and maintainer and checks it against the
// trust store; in enforce mode (or with --strict) an untrusted package fails the
// transaction. Official-repo packages are not ours to gate and are skipped.
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

			type resolved struct {
				pkg    aurweb.Pkg
				source string
			}
			found := make(map[string]resolved, len(names))
			var unknown []string
			for _, name := range names {
				if pkg, source, ok := comp.Resolve(ctx, name); ok {
					found[name] = resolved{pkg, source}
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
				if sync, serr := syncPackages(); serr == nil {
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
						found[p.Name] = resolved{p, "aur"}
					}
				}
			}

			out := cmd.OutOrStdout()
			blocked := false
			for _, name := range names {
				r, ok := found[name]
				if !ok {
					continue // official-repo package, or unknown to the AUR
				}
				v := store.Evaluate(r.source, r.pkg.PackageBase, r.pkg.Maintainer)
				if v.Decision == trust.Trusted {
					fmt.Fprintf(out, "  ok      %s (%s)\n", name, r.source)
					continue
				}
				fmt.Fprintf(out, "  REVIEW  %s (%s) — %s\n", name, r.source, strings.Join(v.Reasons, "; "))
				blocked = true
			}

			if blocked && enforce {
				return utils.NewErr("untrusted packages in transaction; review with 'kayo update' or 'kayo trust add'")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "fail the transaction even in warn mode")
	return cmd
}

// syncPackages returns the set of package names any sync (official) repo
// provides, via `pacman -Slq`, used to tell official packages from foreign ones.
func syncPackages() (map[string]bool, error) {
	out, err := exec.Command("pacman", "-Slq").Output()
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, n := range strings.Fields(string(out)) {
		set[n] = true
	}
	return set, nil
}
