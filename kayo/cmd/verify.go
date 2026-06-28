package cmd

import (
	"fmt"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/pacmanhook"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/kayo/trust"
	"github.com/spf13/cobra"
)

// verifyCmd is the install-time backstop the pacman PreTransaction hook calls.
// It resolves each target to its source and maintainer and checks it against the
// trust store; in enforce mode (or with --strict) an untrusted package fails the
// transaction. Packages no source knows (official-repo packages) are ignored.
func verifyCmd() *cobra.Command {
	var strict bool
	cmd := &cobra.Command{
		Use:   "verify [pkgname...]",
		Short: "Check that packages being installed are trusted (pacman hook entry point)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			names := args
			if len(names) == 0 {
				names = pacmanhook.StdinTargets() // pacman NeedsTargets passes targets on stdin
			}
			if len(names) == 0 {
				return nil
			}

			store, err := trust.Open(cfg.ResolvedTrustStore())
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			comp, err := buildComposite(ctx, cfg)
			if err != nil {
				return err
			}
			up := upstreamClient(cfg)

			out := cmd.OutOrStdout()
			blocked := false
			for _, name := range names {
				pkg, source, ok := comp.Resolve(ctx, name)
				if !ok && up != nil {
					if ps, e := up.Info(ctx, []string{name}); e == nil {
						for _, p := range ps {
							if p.Name == name {
								pkg, source, ok = p, "aur", true
								break
							}
						}
					}
				}
				if !ok {
					continue // not managed by any source (e.g. an official-repo package)
				}
				v := store.Evaluate(source, pkg.PackageBase, pkg.Maintainer)
				if v.Decision == trust.Trusted {
					fmt.Fprintf(out, "  ok      %s (%s)\n", name, source)
					continue
				}
				fmt.Fprintf(out, "  REVIEW  %s (%s) — %s\n", name, source, strings.Join(v.Reasons, "; "))
				blocked = true
			}

			if blocked && (strict || cfg.ResolvedEnforceMode() == "enforce") {
				return utils.NewErr("untrusted packages in transaction; review with 'kayo update' or 'kayo trust add'")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "fail the transaction even in warn mode")
	return cmd
}
