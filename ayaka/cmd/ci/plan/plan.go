package plancmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/ayaka/cli"
	"github.com/Hayao0819/Kamisato/ayaka/service/plan"
	"github.com/Hayao0819/Kamisato/ayaka/service/source"
	"github.com/Hayao0819/Kamisato/internal/errors"
	pkg "github.com/Hayao0819/Kamisato/internal/pacman"
	"github.com/Hayao0819/Kamisato/internal/pacman/repo"
	pacmansource "github.com/Hayao0819/Kamisato/internal/pacman/source"
)

// planner is the slice of service/plan this command drives.
type planner interface {
	Compute(src []*pkg.SourcePackage, rr *repo.RemoteRepo, arch string, cascade plan.CascadeMode, workers int, costs map[string]float64) (*plan.Plan, error)
	ReloadWithSrcinfo(srcrepo *pacmansource.SourceRepo, stderr io.Writer) (*pacmansource.SourceRepo, error)
}

type sourcePlanner struct{}

func (sourcePlanner) Compute(src []*pkg.SourcePackage, rr *repo.RemoteRepo, arch string, cascade plan.CascadeMode, workers int, costs map[string]float64) (*plan.Plan, error) {
	return plan.Compute(src, rr, arch, cascade, workers, costs)
}

func (sourcePlanner) ReloadWithSrcinfo(srcrepo *pacmansource.SourceRepo, stderr io.Writer) (*pacmansource.SourceRepo, error) {
	return source.ReloadWithSrcinfo(srcrepo, stderr)
}

// Cmd computes the build set for one run from the source repo and the published
// repo db alone, so it is idempotent and needs no server-side build state.
func Cmd(runtime *app.Runtime) *cobra.Command { return newCommand(sourcePlanner{}, runtime) }

func newCommand(svc planner, runtime *app.Runtime) *cobra.Command {
	var arch string
	var diffURL string
	var cascade string
	var costsFile string
	var format string
	var workers int
	var updateSrcinfo bool
	cmd := cobra.Command{
		Use:               "plan <srcrepo>",
		Short:             "Compute which packages to build (version diff + rebuild cascade)",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cli.CompleteSrcRepoNames(runtime),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := runtime.App()
			if err != nil {
				return err
			}
			server, err := cmd.Flags().GetString("server")
			if err != nil {
				return err
			}
			mode, err := plan.ParseCascadeMode(cascade)
			if err != nil {
				return err
			}
			if format != "lines" && format != "json" {
				return errors.NewErr("invalid format: " + format + " (lines or json)")
			}

			srcrepo := a.GetSrcRepo(args[0])
			if srcrepo == nil {
				return errors.WrapErr(cli.ErrSourceRepoNotFound, args[0])
			}
			if updateSrcinfo {
				srcrepo, err = svc.ReloadWithSrcinfo(srcrepo, cmd.ErrOrStderr())
				if err != nil {
					return err
				}
			}

			rr, err := cli.RemoteRepo(diffURL, server, srcrepo, arch)
			if err != nil {
				return err
			}

			var costs map[string]float64
			if costsFile != "" {
				data, err := os.ReadFile(costsFile)
				if err != nil {
					return errors.WrapErr(err, "failed to read costs file")
				}
				if err := json.Unmarshal(data, &costs); err != nil {
					return errors.WrapErr(err, "failed to parse costs file")
				}
			}

			p, err := svc.Compute(srcrepo.Pkgs, rr, arch, mode, workers, costs)
			if err != nil {
				return err
			}

			if format == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(p)
			}
			for _, pb := range p.Order {
				fmt.Fprintln(cmd.OutOrStdout(), pb)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Target architecture to plan for")
	cmd.Flags().StringVar(&diffURL, "diff-url", "", "Remote repo db dir (.../repo/<repo>/<arch>); overrides repo.json url")
	cli.AddServerFlag(&cmd)
	_ = cmd.Flags().MarkDeprecated("server", "use --diff-url to point the plan at the remote repo db dir")
	cmd.Flags().StringVar(&cascade, "cascade", "makedepends", "Rebuild propagation: off, makedepends, soname or both")
	cmd.Flags().IntVar(&workers, "workers", 0, "Split the build set into at most N cost-balanced buckets (0 = flat list)")
	cmd.Flags().StringVar(&costsFile, "costs", "", "JSON file of past build times per pkgbase for bucket balancing")
	cmd.Flags().StringVar(&format, "format", "lines", "Output format: lines (pkgbase per line) or json")
	cmd.Flags().BoolVar(&updateSrcinfo, "update-srcinfo", true, "Regenerate .SRCINFO from PKGBUILD before planning (requires makepkg; skipped when absent)")
	return &cmd
}
