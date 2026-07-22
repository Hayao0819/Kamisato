// Package source mutates and reloads source repositories: .SRCINFO
// regeneration, AUR checkouts, pkgrel bumps and repository scaffolding.
package source

import (
	"io"
	"log/slog"
	"os/exec"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// ReloadWithSrcinfo regenerates every .SRCINFO in srcrepo and reloads it so the
// fresh versions drive the diff or plan; returned unchanged when makepkg is
// absent (e.g. CI without pacman tooling).
func ReloadWithSrcinfo(srcrepo *repo.SourceRepo, stderr io.Writer) (*repo.SourceRepo, error) {
	if _, err := exec.LookPath("makepkg"); err != nil {
		slog.Warn("skipping .SRCINFO update: makepkg not found on PATH", "error", err)
		return srcrepo, nil
	}
	if err := RegenerateSrcinfo(srcrepo.Dir, stderr); err != nil {
		return nil, err
	}
	reloaded, err := repo.GetSrcRepo(srcrepo.Dir, srcrepo.Config)
	if err != nil {
		return nil, errors.WrapErr(err, "failed to reload source repo after .SRCINFO update")
	}
	reloaded.Dir = srcrepo.Dir
	reloaded.DestDir = srcrepo.DestDir
	return reloaded, nil
}

// RegenerateSrcinfo rewrites the .SRCINFO of every source package under dir,
// logging (not failing) per-package makepkg errors.
func RegenerateSrcinfo(dir string, stderr io.Writer) error {
	srcdirs, err := repo.GetSrcDirs(dir)
	if err != nil {
		return errors.WrapErr(err, "failed to list source directories")
	}
	for _, d := range srcdirs {
		if err := repo.GenerateSrcinfo(d, stderr); err != nil {
			slog.Warn("failed to update .SRCINFO", "dir", d, "error", err)
		}
	}
	return nil
}

// RegenerateSrcinfoStrict rewrites the .SRCINFO of every source package under
// dir like RegenerateSrcinfo, but fails on the first makepkg error instead of
// warning, and reports each regenerated dir through onUpdated.
func RegenerateSrcinfoStrict(dir string, stderr io.Writer, onUpdated func(dir string)) error {
	srcdirs, err := repo.GetSrcDirs(dir)
	if err != nil {
		return errors.WrapErr(err, "failed to list source directories")
	}
	for _, d := range srcdirs {
		if err := repo.GenerateSrcinfo(d, stderr); err != nil {
			return err
		}
		if onUpdated != nil {
			onUpdated(d)
		}
	}
	return nil
}
