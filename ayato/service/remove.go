package service

import (
	"fmt"
	"log/slog"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// RemovePkg removes a package from the repository, mirroring UploadFile in
// reverse and following the pkgctl/dbscripts model where the architecture scopes
// the removal:
//
//   - arch == "" or "any": de-register the package from every architecture
//     database that lists it (the blinky-compatible route carries no arch and
//     means "remove this package from the repository").
//   - arch == a concrete architecture: de-register from only that arch's database
//     (e.g. drop an arch=any package from x86_64 while leaving aarch64).
//
// The package file is deleted only once no database references it anymore: a
// concrete package's file is unique to its arch and goes with it; an arch=any
// package's shared file under "any/" survives until the last arch drops it.
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

	// Which databases to de-register from. A concrete package lives in exactly one
	// arch; an arch=any package is registered in every configured arch.
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

	// Keep the shared file of an arch=any package that another arch still lists
	// (a per-arch removal). A concrete package's file always goes with its arch.
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

// stillRegistered reports whether pkgname remains in any configured arch database
// outside the just-removed set — i.e. whether a shared (arch=any) file is still
// in use and must be kept.
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

// deleteSignatureIfPresent removes "<pkgFile>.sig" when it exists, best-effort. A
// package may legitimately be unsigned, so a missing signature is not an error
// (blinky removes the .sig and tolerates its absence); listing first avoids the
// blob backend logging a spurious delete failure for unsigned packages.
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
