// Package source mutates and reloads source repositories: .SRCINFO
// regeneration, AUR checkouts, pkgrel bumps and repository scaffolding.
package source

import (
	"context"
	"io"
	"log/slog"
	"os/exec"

	"github.com/Hayao0819/Kamisato/ayaka/source"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// ReloadWithSrcinfo regenerates every .SRCINFO in srcrepo and reloads it so the
// fresh versions drive the diff or plan; returned unchanged when makepkg is
// absent (e.g. CI without pacman tooling).
func ReloadWithSrcinfo(ctx context.Context, srcrepo *source.SourceRepo, stderr io.Writer) (*source.SourceRepo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := exec.LookPath("makepkg"); err != nil {
		slog.Warn("skipping .SRCINFO update: makepkg not found on PATH", "error", err)
		return srcrepo, nil
	}
	if err := RegenerateSrcinfo(ctx, srcrepo.Dir, stderr); err != nil {
		return nil, err
	}
	reloaded, err := source.GetSrcRepo(srcrepo.Dir, srcrepo.Config)
	if err != nil {
		return nil, errors.WrapErr(err, "failed to reload source repo after .SRCINFO update")
	}
	reloaded.Dir = srcrepo.Dir
	reloaded.DestDir = srcrepo.DestDir
	return reloaded, nil
}

// RegenerateSrcinfo rewrites the .SRCINFO of every source package under dir,
// logging (not failing) per-package makepkg errors.
func RegenerateSrcinfo(ctx context.Context, dir string, stderr io.Writer) error {
	srcdirs, err := source.GetSrcDirs(dir)
	if err != nil {
		return errors.WrapErr(err, "failed to list source directories")
	}
	for _, d := range srcdirs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := source.GenerateSrcinfo(ctx, d, stderr); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Warn("failed to update .SRCINFO", "dir", d, "error", err)
		}
	}
	return nil
}

// RegenerateSrcinfoStrict rewrites the .SRCINFO of every source package under
// dir like RegenerateSrcinfo, but fails on the first makepkg error instead of
// warning, and reports each regenerated dir through onUpdated.
func RegenerateSrcinfoStrict(ctx context.Context, dir string, stderr io.Writer, onUpdated func(dir string)) error {
	srcdirs, err := source.GetSrcDirs(dir)
	if err != nil {
		return errors.WrapErr(err, "failed to list source directories")
	}
	for _, d := range srcdirs {
		if err := source.GenerateSrcinfo(ctx, d, stderr); err != nil {
			return err
		}
		if onUpdated != nil {
			onUpdated(d)
		}
	}
	return nil
}
