package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/miko/domain"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
)

// On success it returns the output directory holding the built packages; the
// caller owns its cleanup (after signing/uploading).
func (s *Service) runBuild(ctx context.Context, job *domain.BuildJob) (*builder.Result, string, error) {
	req := job.Request

	// Disposable source directory (discarded after the build).
	srcDir, err := os.MkdirTemp("", "miko-src-*")
	if err != nil {
		return nil, "", utils.WrapErr(err, "failed to create source dir")
	}
	defer func() { _ = os.RemoveAll(srcDir) }()

	if err := materialize(req, srcDir); err != nil {
		return nil, "", utils.WrapErr(err, "failed to materialize source")
	}

	// The artifact directory must live beyond runBuild (until signing and
	// upload), so do not remove it here. Clean it up only on failure.
	outDir, err := os.MkdirTemp("", "miko-out-*")
	if err != nil {
		return nil, "", utils.WrapErr(err, "failed to create output dir")
	}

	// Per-request timeout (minutes) overrides the server default.
	timeoutMin := s.cfg.Build.Timeout
	if req.Timeout > 0 {
		timeoutMin = req.Timeout
	}
	timeout := time.Duration(timeoutMin) * time.Minute

	buildRepos := extraRepos(s.cfg.Build.ExtraRepos)
	if s.cfg.Build.ResolveAURDeps && req.Repo != "" && s.cfg.Ayato.URL != "" {
		// Expose the target repo so dependencies published during this run resolve.
		buildRepos = append(buildRepos, builder.RepoSpec{
			Name:     req.Repo,
			Server:   strings.TrimRight(s.cfg.Ayato.URL, "/") + "/repo/$repo/$arch",
			SigLevel: "Optional TrustAll",
		})
	}

	opts := builder.Options{
		Image:      s.cfg.Build.Image,
		Timeout:    timeout,
		DockerHost: s.cfg.DockerHost,
		ExtraRepos: buildRepos,
	}
	if s.cfg.Cache.Enabled {
		opts.PacmanCacheDir = s.cfg.Cache.PacmanCacheDir
		opts.CcacheDir = s.cfg.Cache.CcacheDir
	}
	backend, err := builder.New(builder.Kind(s.cfg.Executor), opts)
	if err != nil {
		_ = os.RemoveAll(outDir)
		return nil, "", utils.WrapErr(err, "failed to create build backend")
	}

	// Build and publish any unbuilt AUR dependencies before the target so it can
	// install them from the exposed repo (no-op unless resolve_aur_deps is set).
	if err := s.resolveAndBuildDeps(ctx, job, backend, srcDir); err != nil {
		_ = os.RemoveAll(outDir)
		return nil, "", err
	}

	spec := builder.Spec{
		SrcDir:      srcDir,
		OutDir:      outDir,
		Arch:        req.Arch,
		ArchBuild:   s.archBuildFor(req.Arch),
		InstallPkgs: req.InstallPkgs,
		LogWriter:   job.Log,
	}

	res, err := backend.Build(ctx, spec)
	if err != nil {
		_ = os.RemoveAll(outDir)
		return nil, "", utils.WrapErr(err, "build failed")
	}
	return res, outDir, nil
}

// extraRepos maps the configured extra repositories to the builder's RepoSpec.
func extraRepos(repos []conf.ExtraRepo) []builder.RepoSpec {
	if len(repos) == 0 {
		return nil
	}
	out := make([]builder.RepoSpec, 0, len(repos))
	for _, r := range repos {
		out = append(out, builder.RepoSpec{Name: r.Name, Server: r.Server, SigLevel: r.SigLevel})
	}
	return out
}

// archBuildFor maps a CARCH to the devtools wrapper used by the chroot backend,
// using the configured ArchBuildTemplate (default "extra-%s-build").
func (s *Service) archBuildFor(arch string) string {
	if arch == "" {
		return ""
	}
	tmpl := s.cfg.ArchBuildTemplate
	if tmpl == "" {
		tmpl = "extra-%s-build"
	}
	return fmt.Sprintf(tmpl, arch)
}
