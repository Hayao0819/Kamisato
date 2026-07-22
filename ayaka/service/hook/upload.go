// Package hook collects the package files a pacman PostTransaction hook must
// upload to ayato.
package hook

import (
	"log/slog"

	"github.com/samber/lo"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/pacman"
	"github.com/Hayao0819/Kamisato/pkg/pacman/makepkgconf"
	pacmanpkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
)

// CollectFiles resolves the on-disk package files for names: by default only
// foreign (AUR/local) packages, unless all is set. Search dirs are
// cacheOverride when set, else buildDirs plus makepkg's PKGDEST plus pacman's
// CacheDir (from pacmanConf, or pacman's own when empty).
func CollectFiles(names []string, all bool, cacheOverride, buildDirs []string, pacmanConf string) ([]string, error) {
	// By default only publish foreign (AUR/local) packages; the hook's
	// Target=* otherwise fires for every official-repo package, which
	// already lives on mirrors.
	if !all {
		foreign, err := pacman.ForeignPackages()
		if err != nil {
			return nil, errors.WrapErr(err, "could not determine foreign packages; pass --all to upload every target")
		}
		names = lo.Filter(names, func(name string, _ int) bool {
			return foreign[name]
		})
		if len(names) == 0 {
			slog.Info("no foreign (AUR/local) packages to upload in this transaction")
			return nil, nil
		}
	}

	// Search build-output dirs (PKGDEST / --build-dir) before the cache:
	// foreign packages live in the former, repo downloads in the latter.
	dirs := cacheOverride
	if len(dirs) == 0 {
		dirs = append([]string{}, buildDirs...)
		if config, err := makepkgconf.Read(); err == nil && config.PKGDEST != "" {
			dirs = append(dirs, config.PKGDEST)
		}
		dirs = append(dirs, pacman.CacheDirs(pacmanConf)...)
	}

	installed, err := pacman.InstalledPackages()
	if err != nil {
		return nil, errors.WrapErr(err, "read installed package versions")
	}
	var files []string
	for _, name := range names {
		pkg, ok := installed[name]
		if !ok {
			slog.Warn("skipping package not in the local db", "name", name)
			continue
		}
		path, ok := pacmanpkg.FindCached(dirs, name, pkg.Version, pkg.Arch)
		if !ok {
			slog.Warn("no package file found; skipping upload (set makepkg PKGDEST or --build-dir for locally-built packages)", "name", name, "version", pkg.Version)
			continue
		}
		files = append(files, path)
	}
	return files, nil
}
