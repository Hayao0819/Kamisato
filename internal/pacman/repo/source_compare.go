package repo

import (
	"log/slog"

	alpm "github.com/Hayao0819/dyalpm"

	"github.com/Hayao0819/Kamisato/internal/pacman/pkg"
)

func DiffPackages(src []*pkg.SourcePackage, remote *RemoteRepo) []*pkg.SourcePackage {
	var toBuild []*pkg.SourcePackage
	for _, source := range src {
		published := remote.PkgByPkgBase(source.Base())
		if published == nil {
			slog.Warn("Package does not exist in remote repository", "pkgbase", source.Base())
			toBuild = append(toBuild, source)
			continue
		}
		if alpm.VerCmp(source.Version(), published.Version()) > 0 {
			slog.Debug("Local package is newer", "pkgbase", source.Base(), "local", source.Version(), "remote", published.Version())
			toBuild = append(toBuild, source)
		}
	}
	return toBuild
}

func PrunablePackages(desired []string, remote *RemoteRepo) []string {
	set := make(map[string]struct{}, len(desired))
	for _, name := range desired {
		set[name] = struct{}{}
	}
	var prune []string
	for _, binary := range remote.Pkgs {
		if _, ok := set[binary.Name()]; !ok {
			prune = append(prune, binary.Name())
		}
	}
	return prune
}
