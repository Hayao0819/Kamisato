package service

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
	"github.com/Hayao0819/Kamisato/internal/pacman/builder/factory"
	"github.com/Hayao0819/Kamisato/miko/domain"
)

// On success it returns the output directory holding the built packages; the
// caller owns its cleanup (after signing/uploading).
func (s *Service) runBuild(ctx context.Context, job *domain.BuildJob) (*builder.Result, string, error) {
	req := job.Request

	srcDir, err := os.MkdirTemp("", "miko-src-*")
	if err != nil {
		return nil, "", errors.WrapErr(err, "failed to create source dir")
	}
	defer func() { _ = os.RemoveAll(srcDir) }()

	if err := materialize(ctx, req, srcDir); err != nil {
		return nil, "", errors.WrapErr(err, "failed to materialize source")
	}

	// The artifact dir must outlive runBuild (signing/upload), so clean it up
	// only on failure.
	outDir, err := os.MkdirTemp("", "miko-out-*")
	if err != nil {
		return nil, "", errors.WrapErr(err, "failed to create output dir")
	}

	// Per-request timeout (minutes) overrides the trusted server default.
	var timeout time.Duration
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Minute
	}

	overrides := builder.BuildOverrides{
		Timeout: timeout,
		Makepkg: builder.MakepkgConfig{Microarch: req.Microarch},
	}
	if s.settings.ResolveAURDependencies && req.Repo != "" && s.settings.AyatoURL != "" {
		// Expose the target repo so dependencies published during this run resolve.
		overrides.Repositories = append(overrides.Repositories, builder.PacmanRepository{
			Name:     req.Repo,
			Server:   strings.TrimRight(s.settings.AyatoURL, "/") + "/repo/$repo/$arch",
			SigLevel: "Optional TrustAll",
		})
	}

	config, err := builder.Resolve(s.settings.Builder, overrides, req.Arch)
	if err != nil {
		_ = os.RemoveAll(outDir)
		return nil, "", errors.WrapErr(err, "failed to resolve build configuration")
	}
	backend, err := factory.New(config)
	if err != nil {
		_ = os.RemoveAll(outDir)
		return nil, "", errors.WrapErr(err, "failed to create build backend")
	}

	// Build and publish any unbuilt AUR dependencies before the target so it can
	// install them from the exposed repo (no-op unless resolve_aur_deps is set).
	if err := s.resolveAndBuildDeps(ctx, job, backend, srcDir); err != nil {
		_ = os.RemoveAll(outDir)
		return nil, "", err
	}

	spec := builder.Spec{
		SrcDir:        srcDir,
		OutDir:        outDir,
		Arch:          req.Arch,
		InstallPkgs:   req.InstallPkgs,
		LogWriter:     s.LogBuffer(job.ID),
		IgnoreArch:    req.IgnoreArch,
		RunCheck:      req.RunCheck,
		RunVerify:     req.RunVerify,
		SkipChecksums: req.SkipChecksums,
		SkipPGPCheck:  req.SkipPGPCheck,
	}

	res, err := backend.Build(ctx, spec)
	if err != nil {
		_ = os.RemoveAll(outDir)
		return nil, "", errors.WrapErr(err, "build failed")
	}
	return res, outDir, nil
}
