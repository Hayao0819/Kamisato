package service

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/miko/domain"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	"github.com/Hayao0819/Kamisato/pkg/pacman/depsolve"
)

// resolveAndBuildDeps resolves a target's unbuilt AUR dependencies, builds each in
// dependency order and publishes it to ayato so the target build can install it.
// It is a no-op unless enabled by config. Dependencies are read from the source's
// .SRCINFO (never by sourcing the untrusted PKGBUILD on the host); a source
// without a .SRCINFO is skipped.
func (s *Service) resolveAndBuildDeps(ctx context.Context, job *domain.BuildJob, backend builder.Backend, srcDir string) error {
	if !s.cfg.Build.ResolveAURDeps {
		return nil
	}

	data, err := os.ReadFile(filepath.Join(srcDir, ".SRCINFO"))
	if err != nil {
		slog.Warn("resolve_aur_deps is on but the source has no .SRCINFO; skipping AUR dependency resolution", "err", err)
		return nil
	}
	rootDeps := srcinfoBuildDeps(data, job.Request.Arch)
	if len(rootDeps) == 0 {
		return nil
	}

	up := aurweb.NewAURUpstream(s.cfg.Build.AURRPCURL)
	order, err := depsolve.Resolve(ctx, rootDeps, NewRepoChecker(), NewAURSource(up))
	if err != nil {
		return utils.WrapErr(err, "failed to resolve AUR dependencies")
	}
	if len(order) == 0 {
		return nil
	}

	slog.Info("building AUR dependencies before target", "count", len(order), "repo", job.Request.Repo)
	for _, dep := range order {
		if err := s.buildAndPublishDep(ctx, job, backend, up, dep); err != nil {
			return utils.WrapErr(err, "failed to build AUR dependency "+dep.PackageBase)
		}
	}
	return nil
}

// buildAndPublishDep clones one AUR dependency's PKGBUILD, builds it with the same
// backend and target arch, and publishes it to the target's repo. Once published,
// the target repo (exposed to every build in this run) makes it installable.
func (s *Service) buildAndPublishDep(ctx context.Context, job *domain.BuildJob, backend builder.Backend, up *aurweb.AURUpstream, dep depsolve.Pkg) error {
	depSrc, err := os.MkdirTemp("", "miko-dep-*")
	if err != nil {
		return utils.WrapErr(err, "failed to create dependency source dir")
	}
	defer func() { _ = os.RemoveAll(depSrc) }()

	gitURL := strings.TrimRight(up.GitBase(), "/") + "/" + dep.PackageBase + ".git"
	if err := materializeGit(&domain.GitSource{URL: gitURL}, depSrc); err != nil {
		return utils.WrapErr(err, "failed to clone AUR dependency")
	}

	depOut, err := os.MkdirTemp("", "miko-depout-*")
	if err != nil {
		return utils.WrapErr(err, "failed to create dependency output dir")
	}
	defer func() { _ = os.RemoveAll(depOut) }()

	spec := builder.Spec{
		SrcDir:    depSrc,
		OutDir:    depOut,
		Arch:      job.Request.Arch,
		ArchBuild: s.archBuildFor(job.Request.Arch),
		LogWriter: job.Log,
	}
	res, err := backend.Build(ctx, spec)
	if err != nil {
		return utils.WrapErr(err, "dependency build failed")
	}
	return s.signAndUpload(ctx, job.Request.Repo, res.Packages)
}

// srcinfoBuildDeps extracts the build-relevant dependency specs (depends,
// makedepends, checkdepends, including the arch-specific variants) from .SRCINFO
// content, preserving order and de-duplicating.
func srcinfoBuildDeps(data []byte, arch string) []string {
	want := map[string]bool{}
	for _, base := range []string{"depends", "makedepends", "checkdepends"} {
		want[base] = true
		if arch != "" {
			want[base+"_"+arch] = true
		}
	}
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if val == "" || !want[key] || seen[val] {
			continue
		}
		seen[val] = true
		out = append(out, val)
	}
	return out
}
