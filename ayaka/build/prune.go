package build

import "github.com/Hayao0819/Kamisato/pkg/pacman/repo"

// PrunablePackages returns the pkgnames present in the remote repo that are not in
// desired — i.e. packages whose PKGBUILD was removed from the source repo.
func PrunablePackages(desired []string, rr *repo.RemoteRepo) []string {
	set := make(map[string]struct{}, len(desired))
	for _, n := range desired {
		set[n] = struct{}{}
	}
	var prune []string
	for _, bp := range rr.Pkgs {
		if _, ok := set[bp.Name()]; !ok {
			prune = append(prune, bp.Name())
		}
	}
	return prune
}
