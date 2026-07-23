package matrixcmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/samber/lo"
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/ayaka/service/plan"
	"github.com/Hayao0819/Kamisato/ayaka/service/source"
	"github.com/Hayao0819/Kamisato/internal/errors"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// planner is the slice of service/plan this command drives.
type planner interface {
	Compute(src []*pkg.SourcePackage, rr *repo.RemoteRepo, arch string, cascade plan.CascadeMode, workers int, costs map[string]float64) (*plan.Plan, error)
	ReloadWithSrcinfo(srcrepo *repo.SourceRepo, stderr io.Writer) (*repo.SourceRepo, error)
}

type sourcePlanner struct{}

func (sourcePlanner) Compute(src []*pkg.SourcePackage, rr *repo.RemoteRepo, arch string, cascade plan.CascadeMode, workers int, costs map[string]float64) (*plan.Plan, error) {
	return plan.Compute(src, rr, arch, cascade, workers, costs)
}

func (sourcePlanner) ReloadWithSrcinfo(srcrepo *repo.SourceRepo, stderr io.Writer) (*repo.SourceRepo, error) {
	return source.ReloadWithSrcinfo(srcrepo, stderr)
}

type buildEntry struct {
	Repo string `json:"repo"`
	Arch string `json:"arch"`
	Pkgs string `json:"pkgs"`
}

type pruneEntry struct {
	Repo string `json:"repo"`
	Arch string `json:"arch"`
}

// Matrix is the machine-readable result of `ayaka matrix`: one CI job matrix
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
func Cmd() *cobra.Command { return newCommand(sourcePlanner{}) }

func newCommand(svc planner) *cobra.Command {
	var server string
	var cascade string
	var format string
	var packages string
	var all bool
	var workers int
	var updateSrcinfo bool
	cmd := cobra.Command{
		Use:   "matrix",
		Short: "Compute CI build/prune matrices across all source repos and arches",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := plan.ParseCascadeMode(cascade)
			if err != nil {
				return err
			}
			if format != "json" && format != "github" {
				return errors.NewErr("invalid format: " + format + " (json or github)")
			}
			force := all || packages != ""

			m := Matrix{Bumps: map[string]string{}}
			m.BuildMatrix.Include = []buildEntry{}
			m.PruneMatrix.Include = []pruneEntry{}
			for _, srcrepo := range app.From(cmd).SrcRepos {
				if updateSrcinfo && !force {
					srcrepo, err = svc.ReloadWithSrcinfo(srcrepo, cmd.ErrOrStderr())
					if err != nil {
						return err
					}
				}
				name := srcrepo.Config.Name
				arches := srcrepo.Config.Build.Arches
				if len(arches) == 0 {
					arches = []string{"x86_64"}
				}
				for _, arch := range arches {
					m.PruneMatrix.Include = append(m.PruneMatrix.Include, pruneEntry{Repo: name, Arch: arch})

					// Force mode bypasses the plan: one bucket per repo/arch
					// with the requested packages (empty = every package).
					if force {
						m.BuildMatrix.Include = append(m.BuildMatrix.Include, buildEntry{Repo: name, Arch: arch, Pkgs: packages})
						continue
					}

					diffURL := ""
					if server != "" {
						diffURL = strings.TrimRight(server, "/") + "/repo/" + name + "/" + arch
					}
					rr, err := shared.RemoteRepo(diffURL, "", srcrepo, arch)
					if err != nil {
						return err
					}
					p, err := svc.Compute(srcrepo.Pkgs, rr, arch, mode, workers, nil)
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

			if format == "github" {
				return writeGithubOutput(&m)
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(&m)
		},
	}
	cmd.Flags().StringVar(&server, "server", "", "Ayato base URL; version diffs read <server>/repo/<repo>/<arch> (empty = repo.json url)")
	cmd.Flags().StringVar(&cascade, "cascade", "makedepends", "Rebuild propagation: off, makedepends, soname or both")
	cmd.Flags().IntVar(&workers, "workers", 0, "Split each build set into at most N cost-balanced buckets (0 = one bucket)")
	cmd.Flags().StringVar(&packages, "packages", "", "Force mode: skip planning and build these packages in every repo/arch")
	cmd.Flags().BoolVar(&all, "all", false, "Force mode: skip planning and rebuild every package in every repo/arch")
	cmd.Flags().StringVar(&format, "format", "json", "Output format: json or github ($GITHUB_OUTPUT job outputs)")
	cmd.Flags().BoolVar(&updateSrcinfo, "update-srcinfo", true, "Regenerate .SRCINFO from PKGBUILD before planning (requires makepkg; skipped when absent)")
	return &cmd
}

// writeGithubOutput appends the matrices in workflow-output form to the file
// GitHub Actions points $GITHUB_OUTPUT at.
func writeGithubOutput(m *Matrix) error {
	path := os.Getenv("GITHUB_OUTPUT")
	if path == "" {
		return errors.NewErr("--format github needs $GITHUB_OUTPUT to be set")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
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
