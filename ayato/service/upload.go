package service

import (
	"fmt"
	"io"
	"log/slog"
	"path"
	"strings"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
)

// TODO: support signed DB, check gnupgDir existence
func (s *Service) UploadFile(repo string, files *domain.UploadFiles) error {
	slog.Info("upload pkg file", "file", files.PkgFile.FileName())
	if s.pkgBinaryRepo.VerifyPkgRepo(repo) != nil {
		slog.Warn("repository directory not found", "repo", repo)
		if err := s.initRepo(repo, false, nil); err != nil {
			return fmt.Errorf("init repo err: %s", err.Error())
		}
	}
	pkgFileStream := files.PkgFile
	p, err := pkg.ReadBinaryPackage(pkgFileStream.FileName(), pkgFileStream)
	if err != nil {
		return fmt.Errorf("get pkg from bin err: %s", err.Error())
	}
	pi := p.PKGINFO()
	slog.Info("get pkg from bin", "pkgname", pi.PkgName, "pkgver", pi.PkgVer)

	// Even with RequireSign=false, a present signature is always verified: a bad,
	// untrusted, expired, or revoked sig is rejected; only a missing sig is
	// allowed. Signatures are binary detached (--no-armor).
	hasSig := files.SigFile != nil
	if s.cfg != nil && s.cfg.RequireSign && !hasSig {
		return fmt.Errorf("package signature is required but none was provided")
	}
	if hasSig {
		if s.verifier == nil {
			// A signature is present but there is no trust root to verify it;
			// reject rather than store an unvalidated signature.
			return fmt.Errorf("package signature present but no verify.keyring is configured to validate it")
		}
		if _, err := pkgFileStream.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("seek pkg file for verify err: %s", err.Error())
		}
		if _, err := files.SigFile.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("seek sig file for verify err: %s", err.Error())
		}
		fpr, verr := s.verifier.VerifyDetached(pkgFileStream, files.SigFile)
		if verr != nil {
			return fmt.Errorf("package signature verification failed: %s", verr.Error())
		}
		slog.Info("package signature verified", "pkgname", pi.PkgName, "fingerprint", fpr)
	}

	// pi.Arch comes from attacker-controlled .PKGINFO; reject anything that is not
	// a single safe path component so it cannot escape the repo dir as a storage
	// subdirectory.
	if pi.Arch == "" || strings.ContainsRune(pi.Arch, '/') || strings.Contains(pi.Arch, "..") {
		return fmt.Errorf("invalid package arch %q", pi.Arch)
	}

	// storeArch is where the file physically lives: an arch=any package is stored
	// once under "any/" and shared via FetchFile's fallback. dbArches are the
	// arches whose database registers the package.
	storeArch := pi.Arch
	dbArches, err := s.targetArches(repo, pi.Arch)
	if err != nil {
		return err
	}

	storedName := path.Base(pkgFileStream.FileName())
	storedSigName := storedName + ".sig"
	useSignedDB := false // TODO: support signed DB
	var gnupgDir *string // TODO: check gnupgDir existence
	var added []string
	sigStored := false
	cleanup := func() {
		for _, arch := range added {
			if rmErr := s.pkgBinaryRepo.RepoRemove(repo, arch, pi.PkgName, useSignedDB, gnupgDir); rmErr != nil {
				slog.Warn("failed to roll back repo-add", "repo", repo, "arch", arch, "pkg", pi.PkgName, "err", rmErr)
			}
		}
		if delErr := s.pkgBinaryRepo.DeleteFile(repo, storeArch, storedName); delErr != nil {
			slog.Warn("failed to clean up stored pkg file after upload error", "repo", repo, "arch", storeArch, "filename", storedName, "err", delErr)
		}
		if sigStored {
			if delErr := s.pkgBinaryRepo.DeleteFile(repo, storeArch, storedSigName); delErr != nil {
				slog.Warn("failed to clean up stored sig file after upload error", "repo", repo, "arch", storeArch, "filename", storedSigName, "err", delErr)
			}
		}
	}

	if _, err := pkgFileStream.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek pkg file err: %s", err.Error())
	}
	if err := s.pkgBinaryRepo.StoreFile(repo, storeArch, pkgFileStream); err != nil {
		return fmt.Errorf("store file err: %s", err.Error())
	}

	// StoreFile keys the on-disk name off FileName(), so re-wrap the sig under
	// "<storedName>.sig". Verification above already rejected unverifiable sigs.
	if hasSig {
		if _, err := files.SigFile.Seek(0, io.SeekStart); err != nil {
			cleanup()
			return fmt.Errorf("seek sig file err: %s", err.Error())
		}
		sigToStore := stream.NewFileStream(storedSigName, files.SigFile.ContentType(), files.SigFile)
		if err := s.pkgBinaryRepo.StoreFile(repo, storeArch, sigToStore); err != nil {
			cleanup()
			return fmt.Errorf("store sig file err: %s", err.Error())
		}
		sigStored = true
	}

	for _, arch := range dbArches {
		if _, err := pkgFileStream.Seek(0, io.SeekStart); err != nil {
			cleanup()
			return fmt.Errorf("seek pkg file err: %s", err.Error())
		}
		if err := s.pkgBinaryRepo.RepoAdd(repo, arch, pkgFileStream, nil, useSignedDB, gnupgDir); err != nil {
			cleanup()
			return fmt.Errorf("repo-add err: %s", err.Error())
		}
		added = append(added, arch)
	}

	if err := s.pkgNameRepo.StorePackageFile(storeArch, pi.PkgName, storedName); err != nil {
		cleanup()
		return fmt.Errorf("store pkg file name err: %s", err.Error())
	}
	return nil
}

// configuredArches returns the repo's concrete arches, dropping "" and "any"
// (pacman has no os/any database; an arch=any package is registered in each
// concrete arch instead).
func (s *Service) configuredArches(repo string) []string {
	rc := s.cfg.Repo(repo)
	if rc == nil {
		return nil
	}
	out := make([]string, 0, len(rc.Arches))
	for _, a := range rc.Arches {
		if a != "" && a != "any" {
			out = append(out, a)
		}
	}
	return out
}

// A concrete arch maps to itself; arch=any expands to every configured arch
// because pacman has no os/any database, so an any package must be in each
// arch's db to be installable.
func (s *Service) targetArches(repo, pkgArch string) ([]string, error) {
	if pkgArch != "any" {
		return []string{pkgArch}, nil
	}
	arches := s.configuredArches(repo)
	if len(arches) == 0 {
		return nil, fmt.Errorf("repository %q has no architectures configured for an arch=any package", repo)
	}
	return arches, nil
}
