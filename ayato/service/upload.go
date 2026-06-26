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

// UploadFile uploads a package file and updates the DB and metadata.
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

	// Cryptographic signature gate. Default policy (RequireSign=false) still
	// always verifies a signature when one is present: a present-but-bad,
	// untrusted, unknown, expired, or revoked signature is rejected regardless
	// of RequireSign. A missing signature is allowed only when RequireSign is
	// false. The .pkg.tar.zst was signed binary detached (--no-armor), so this
	// uses CheckDetachedSignature.
	hasSig := files.SigFile != nil
	if s.cfg != nil && s.cfg.RequireSign && !hasSig {
		return fmt.Errorf("package signature is required but none was provided")
	}
	if hasSig {
		if s.verifier == nil {
			// A signature is present but we have no trust root to check it
			// against. We cannot let an unverifiable signature pass, so reject
			// the upload rather than store a signature we never validated.
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

	// pi.Arch comes from the uploaded package's .PKGINFO and is attacker
	// controlled; reject anything that is not a single safe path component so it
	// cannot escape the repo directory when used as a storage subdirectory.
	if pi.Arch == "" || strings.ContainsRune(pi.Arch, '/') || strings.Contains(pi.Arch, "..") {
		return fmt.Errorf("invalid package arch %q", pi.Arch)
	}

	// storeArch is where the file physically lives: an arch=any package is stored
	// once under "any/" and shared by every arch via FetchFile's fallback. dbArches
	// are the concrete architectures whose database registers the package.
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

	// Persist the verified signature alongside the package as
	// "<storedName>.sig". StoreFile keys the on-disk name off FileName(), so the
	// sig stream is re-wrapped under that exact name. Verification above already
	// rejected an unverifiable sig, so anything we reach here is trusted.
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

// configuredArches returns the repo's configured concrete architectures, with ""
// and "any" filtered out (pacman has no os/any database; an arch=any package is
// registered in each concrete arch instead). It is the single arch-expansion
// helper shared by upload, remove, and the .db read-through.
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

// targetArches resolves the architecture databases an uploaded package is
// registered in. A concrete arch maps to itself. arch=any expands to every
// configured arch of the repo, because pacman has no os/any database: an any
// package must be in each arch's db to be installable there (its file is shared
// from the "any/" directory by FetchFile).
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
