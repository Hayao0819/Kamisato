package plancmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/completion"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/remote"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/sourcerepos"
	"github.com/Hayao0819/Kamisato/ayaka/service/plan"
	"github.com/Hayao0819/Kamisato/ayaka/service/source"
	sourcerepo "github.com/Hayao0819/Kamisato/ayaka/source"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/pacman/pkg"
	"github.com/Hayao0819/Kamisato/internal/pacman/repo"
	"github.com/spf13/cobra"
)

// computeFunc is the planning operation this command drives.
type computeFunc func(ctx context.Context, src []*pkg.SourcePackage, rr *repo.RemoteRepo, arch string, cascade plan.CascadeMode, workers int, costs map[string]float64) (*plan.Plan, error)
type reloadFunc func(ctx context.Context, srcrepo *sourcerepo.SourceRepo, stderr io.Writer) (*sourcerepo.SourceRepo, error)

// Cmd computes the build set from source metadata, the published repo db and
// live VCS refs without separate build state.
func Cmd() *cobra.Command {
	return newCommand(plan.Compute, source.ReloadWithSrcinfo, nil)
}

func newCommand(compute computeFunc, reload reloadFunc, sources sourcerepos.Reader) *cobra.Command {
	var options Options
	var cmd cobra.Command
	if sources == nil {
		sources = sourcerepos.ForCommand(&cmd)
	}
	cmd = cobra.Command{
		Use:               "plan <srcrepo>",
		Short:             "Compute which packages to build (version/VCS diff + rebuild cascade)",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.CompleteSrcRepoNames(sources),
		RunE: func(cmd *cobra.Command, args []string) error {
			server, err := cmd.Flags().GetString("server")
			if err != nil {
				return err
			}
			options.Repository = args[0]
			options.Server = server
			return run(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), sources.Find, compute, reload, options)
		},
	}
	cmd.Flags().StringVar(&options.Arch, "arch", "x86_64", "Target architecture to plan for")
	cmd.Flags().StringVar(&options.DatabaseURL, "diff-url", "", "Remote repo db dir (.../repo/<repo>/<arch>); overrides repo.json url")
	remote.AddServerFlag(&cmd)
	_ = cmd.Flags().MarkDeprecated("server", "use --diff-url to point the plan at the remote repo db dir")
	cmd.Flags().StringVar(&options.Cascade, "cascade", "makedepends", "Rebuild propagation: off, makedepends, soname or both")
	cmd.Flags().IntVar(&options.Workers, "workers", 0, "Split the build set into at most N cost-balanced buckets (0 = flat list)")
	cmd.Flags().StringVar(&options.CostsFile, "costs", "", "JSON file of past build times per pkgbase for bucket balancing")
	cmd.Flags().StringVar(&options.Format, "format", "lines", "Output format: lines (pkgbase per line) or json")
	cmd.Flags().BoolVar(&options.UpdateSrcinfo, "update-srcinfo", true, "Regenerate .SRCINFO from PKGBUILD before planning (requires makepkg; skipped when absent)")
	return &cmd
}

// Options is the parsed input to this command, independent of Cobra.
type Options struct {
	Repository    string
	Arch          string
	DatabaseURL   string
	Server        string
	Cascade       string
	CostsFile     string
	Format        string
	Workers       int
	UpdateSrcinfo bool
}

func run(ctx context.Context, out, stderr io.Writer, resolve sourcerepos.LookupFunc, compute computeFunc, reload reloadFunc, options Options) error {
	mode, err := plan.ParseCascadeMode(options.Cascade)
	if err != nil {
		return err
	}
	if options.Format != "lines" && options.Format != "json" {
		return errors.NewErr("invalid format: " + options.Format + " (lines or json)")
	}

	srcrepo, err := sourcerepos.Require(resolve, options.Repository)
	if err != nil {
		return err
	}
	if options.UpdateSrcinfo {
		srcrepo, err = reload(ctx, srcrepo, stderr)
		if err != nil {
			return err
		}
	}

	rr, err := plan.RemoteRepo(ctx, remote.DatabaseClient(), options.DatabaseURL, options.Server, srcrepo, options.Arch)
	if err != nil {
		return err
	}

	var costs map[string]float64
	if options.CostsFile != "" {
		data, err := os.ReadFile(options.CostsFile)
		if err != nil {
			return errors.WrapErr(err, "failed to read costs file")
		}
		if err := json.Unmarshal(data, &costs); err != nil {
			return errors.WrapErr(err, "failed to parse costs file")
		}
	}

	p, err := compute(ctx, srcrepo.Pkgs, rr, options.Arch, mode, options.Workers, costs)
	if err != nil {
		return err
	}

	if options.Format == "json" {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(p)
	}
	for _, pb := range p.Order {
		fmt.Fprintln(out, pb)
	}
	return nil
}
