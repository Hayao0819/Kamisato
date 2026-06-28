package service

import (
	"fmt"
	"log/slog"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// arch "" or "any" de-registers the package from every arch database; a concrete
// arch de-registers only that one. The file is deleted once no database
// references it: a concrete file goes with its arch, an arch=any file shared
// under "any/" survives until the last arch drops it.
func (s *Service) RemovePkg(rname string, arch string, pkgname string) error {
	if err := s.ValidateRepoName(rname); err != nil {
		slog.Error("validate repo name failed", "repo", rname, "error", err.Error())
		return utils.WrapErr(err, "validate repo name failed")
	}

	filename, storeArch, err := s.resolvePackage(rname, arch, pkgname)
	if err != nil {
		return utils.WrapErr(err, "resolve package")
	}

	allArches := arch == "" || arch == "any"

	// A concrete package lives in exactly one arch; an arch=any package is
	// registered in every configured arch.
	var dbArches []string
	if storeArch == "any" {
		if allArches {
			dbArches = s.configuredArches(rname)
		} else {
			dbArches = []string{arch}
		}
	} else {
		if !allArches && arch != storeArch {
			return fmt.Errorf("package %q is %s in %s, not %s", pkgname, storeArch, rname, arch)
		}
		dbArches = []string{storeArch}
	}
	if len(dbArches) == 0 {
		return fmt.Errorf("no architecture database to remove %q from in %s", pkgname, rname)
	}

	useSignedDB := false // TODO: support signed DB
	var gnupgDir *string
	for _, a := range dbArches {
		if err := s.pkgBinaryRepo.RepoRemove(rname, a, pkgname, useSignedDB, gnupgDir); err != nil {
			return utils.WrapErr(err, fmt.Sprintf("repo-remove %s from %s/%s", pkgname, rname, a))
		}
	}

	// Keep the shared arch=any file while another arch still lists it; a concrete
	// file always goes with its arch.
	if storeArch == "any" && !allArches && s.stillRegistered(rname, pkgname, dbArches) {
		return nil
	}

	if err := s.pkgBinaryRepo.DeleteFile(rname, storeArch, filename); err != nil {
		return utils.WrapErr(err, "delete package file")
	}
	s.deleteSignatureIfPresent(rname, storeArch, filename)
	if err := s.pkgNameRepo.DeletePackageFileEntry(storeArch, pkgname); err != nil {
		return utils.WrapErr(err, "delete package metadata entry")
	}
	return nil
}

// stillRegistered reports whether a shared arch=any file is still in use (listed
// by an arch outside the just-removed set) and must be kept.
func (s *Service) stillRegistered(repo, pkgname string, removed []string) bool {
	removedSet := make(map[string]struct{}, len(removed))
	for _, a := range removed {
		removedSet[a] = struct{}{}
	}
	for _, a := range s.configuredArches(repo) {
		if _, ok := removedSet[a]; ok {
			continue
		}
		rr, err := s.pkgBinaryRepo.RemoteRepo(repo, a)
		if err != nil {
			continue
		}
		if rr.PkgByPkgName(pkgname) != nil {
			return true
		}
	}
	return false
}

// deleteSignatureIfPresent best-effort removes "<pkgFile>.sig". A package may be
// unsigned, so a missing sig is not an error; listing first avoids the blob
// backend logging a spurious delete failure.
func (s *Service) deleteSignatureIfPresent(repo, arch, pkgFile string) {
	sig := pkgFile + ".sig"
	files, err := s.pkgBinaryRepo.Files(repo, arch)
	if err != nil {
		slog.Warn("list files to locate signature", "repo", repo, "arch", arch, "err", err)
		return
	}
	for _, f := range files {
		if f == sig {
			if err := s.pkgBinaryRepo.DeleteFile(repo, arch, sig); err != nil {
				slog.Warn("delete package signature", "repo", repo, "arch", arch, "file", sig, "err", err)
			}
			return
		}
	}
}
