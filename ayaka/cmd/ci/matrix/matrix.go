package matrixcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/remote"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/sourcerepos"
	"github.com/Hayao0819/Kamisato/ayaka/service/plan"
	"github.com/Hayao0819/Kamisato/ayaka/service/source"
	sourcerepo "github.com/Hayao0819/Kamisato/ayaka/source"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/pacman/pkg"
	"github.com/Hayao0819/Kamisato/internal/pacman/repo"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

// computeFunc is the planning operation this command drives.
type computeFunc func(ctx context.Context, src []*pkg.SourcePackage, rr *repo.RemoteRepo, arch string, cascade plan.CascadeMode, workers int, costs map[string]float64) (*plan.Plan, error)
type reloadFunc func(ctx context.Context, srcrepo *sourcerepo.SourceRepo, stderr io.Writer) (*sourcerepo.SourceRepo, error)

type buildEntry struct {
	Repo string `json:"repo"`
	Arch string `json:"arch"`
	Pkgs string `json:"pkgs"`
}

type pruneEntry struct {
	Repo string `json:"repo"`
	Arch string `json:"arch"`
}

// Matrix is the machine-readable result of `ayaka ci matrix`: one CI job matrix
// for builds (one entry per plan bucket), one for prunes (every repo/arch),
// and the pkgrel bump targets unioned per repo (pkgrel is arch-independent).
type Matrix struct {
	BuildMatrix struct {
		Include []buildEntry `json:"include"`
	} `json:"build_matrix"`
	PruneMatrix struct {
		Include []pruneEntry `json:"include"`
	} `json:"prune_matrix"`
	Bumps    map[string]string `json:"bumps"`
	AnyBuild bool              `json:"any_build"`
}

// Cmd aggregates `ayaka ci plan` over every configured source repo and its
// repo.json arches into the job matrices a CI run consumes, so the workflow
// side needs no jq assembly.
func Cmd() *cobra.Command {
	return newCommand(plan.Compute, source.ReloadWithSrcinfo, nil)
}

func newCommand(compute computeFunc, reload reloadFunc, list sourcerepos.ListFunc) *cobra.Command {
	var options Options
	var cmd cobra.Command
	if list == nil {
		list = sourcerepos.ForCommand(&cmd).All
	}
	cmd = cobra.Command{
		Use:   "matrix",
		Short: "Compute CI build/prune matrices across all source repos and arches",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), list, compute, reload, options)
		},
	}
	cmd.Flags().StringVar(&options.Server, "server", "", "Ayato base URL; version diffs read <server>/repo/<repo>/<arch> (empty = repo.json url)")
	cmd.Flags().StringVar(&options.Cascade, "cascade", "makedepends", "Rebuild propagation: off, makedepends, soname or both")
	cmd.Flags().IntVar(&options.Workers, "workers", 0, "Split each build set into at most N cost-balanced buckets (0 = one bucket)")
	cmd.Flags().StringVar(&options.Packages, "packages", "", "Force mode: skip planning and build these packages in every repo/arch")
	cmd.Flags().BoolVar(&options.All, "all", false, "Force mode: skip planning and rebuild every package in every repo/arch")
	cmd.Flags().StringVar(&options.Format, "format", "json", "Output format: json or github ($GITHUB_OUTPUT job outputs)")
	cmd.Flags().BoolVar(&options.UpdateSrcinfo, "update-srcinfo", true, "Regenerate .SRCINFO from PKGBUILD before planning (requires makepkg; skipped when absent)")
	return &cmd
}

// writeGithubOutput appends the matrices in workflow-output form to the file
// GitHub Actions points $GITHUB_OUTPUT at.
func writeGithubOutput(m *Matrix) error {
	path := os.Getenv("GITHUB_OUTPUT")
	if path == "" {
		return errors.NewErr("--format github needs $GITHUB_OUTPUT to be set")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // G703: $GITHUB_OUTPUT is the runner-provided output path by contract
	if err != nil {
		return errors.WrapErr(err, "failed to open $GITHUB_OUTPUT")
	}
	defer f.Close()

	buildJSON, err := json.Marshal(m.BuildMatrix)
	if err != nil {
		return err
	}
	pruneJSON, err := json.Marshal(m.PruneMatrix)
	if err != nil {
		return err
	}
	bumpsJSON, err := json.Marshal(m.Bumps)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "build_matrix=%s\nprune_matrix=%s\nbumps=%s\nany_build=%t\n", buildJSON, pruneJSON, bumpsJSON, m.AnyBuild)
	return err
}

// Options is the parsed input to this command, independent of Cobra.
type Options struct {
	Server        string
	Cascade       string
	Format        string
	Packages      string
	All           bool
	Workers       int
	UpdateSrcinfo bool
}

func run(ctx context.Context, out, stderr io.Writer, list sourcerepos.ListFunc, compute computeFunc, reload reloadFunc, options Options) error {
	repos, err := list()
	if err != nil {
		return err
	}
	mode, err := plan.ParseCascadeMode(options.Cascade)
	if err != nil {
		return err
	}
	if options.Format != "json" && options.Format != "github" {
		return errors.NewErr("invalid format: " + options.Format + " (json or github)")
	}
	force := options.All || options.Packages != ""

	m := Matrix{Bumps: map[string]string{}}
	m.BuildMatrix.Include = []buildEntry{}
	m.PruneMatrix.Include = []pruneEntry{}
	for _, srcrepo := range repos {
		if options.UpdateSrcinfo && !force {
			srcrepo, err = reload(ctx, srcrepo, stderr)
			if err != nil {
				return err
			}
		}
		name := srcrepo.Config.Name
		arches := srcrepo.Config.Build.ArchNames()
		if len(arches) == 0 {
			arches = []string{"x86_64"}
		}
		for _, arch := range arches {
			m.PruneMatrix.Include = append(m.PruneMatrix.Include, pruneEntry{Repo: name, Arch: arch})

			// Force mode bypasses the plan: one bucket per repo/arch
			// with the requested packages (empty = every package).
			if force {
				m.BuildMatrix.Include = append(m.BuildMatrix.Include, buildEntry{Repo: name, Arch: arch, Pkgs: options.Packages})
				continue
			}

			diffURL := ""
			if options.Server != "" {
				diffURL = strings.TrimRight(options.Server, "/") + "/repo/" + name + "/" + arch
			}
			rr, err := plan.RemoteRepo(ctx, remote.DatabaseClient(), diffURL, "", srcrepo, arch)
			if err != nil {
				return err
			}
			p, err := compute(ctx, srcrepo.Pkgs, rr, arch, mode, options.Workers, nil)
			if err != nil {
				return err
			}
			slog.Info("planned", "repo", name, "arch", arch, "order", p.Order, "bumps", p.BumpTargets)

			buckets := p.Buckets
			if len(buckets) == 0 && len(p.Order) > 0 {
				buckets = [][]string{p.Order}
			}
			for _, b := range buckets {
				m.BuildMatrix.Include = append(m.BuildMatrix.Include, buildEntry{Repo: name, Arch: arch, Pkgs: strings.Join(b, " ")})
			}
			if len(p.BumpTargets) > 0 {
				merged := append(strings.Fields(m.Bumps[name]), p.BumpTargets...)
				m.Bumps[name] = strings.Join(lo.Uniq(merged), " ")
			}
		}
	}
	m.AnyBuild = len(m.BuildMatrix.Include) > 0
	if !m.AnyBuild {
		slog.Info("no packages need building")
	}

	if options.Format == "github" {
		return writeGithubOutput(&m)
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(&m)
}
