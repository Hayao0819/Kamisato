package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/pkg/pacman/alpm"
)

// Promoter advances a package through a tiered repo's staging -> testing -> stable
// flow. Building (miko) and publishing to a tier stay separate: an uploaded
// package is published to staging and only reaches testing/stable by an explicit
// PromotePackage.
type Promoter interface {
	PromotePackage(ctx context.Context, repo string, from, to domain.Tier, pkgname, version string) error
}

// PromotePackage moves a package to the next tier of a tiered repo. Promotion is a
// copy + DB op, never a re-upload: it copies the built package into the target tier
// and registers it via the CAS commit, so the tier's db gains the package
// atomically. The source tier keeps or drops it per policy.
func (s *Service) PromotePackage(ctx context.Context, repo string, from, to domain.Tier, pkgname, version string) error {
	if s.catalog == nil {
		return fmt.Errorf("%w: promotion requires a repository configuration", domain.ErrInvalid)
	}
	rc, ok := s.catalog.Logical(repo)
	if !ok || !rc.Tiered() {
		return fmt.Errorf("%w: %q is not a tiered repository", domain.ErrInvalid, repo)
	}
	if !domain.CanPromote(from, to) {
		return fmt.Errorf("%w: cannot promote %q from %q to %q (allowed: staging->testing, testing->stable)", domain.ErrInvalid, pkgname, from, to)
	}
	src, err := rc.PhysicalName(from)
	if err != nil {
		return err
	}
	dst, err := rc.PhysicalName(to)
	if err != nil {
		return err
	}
	releasePublication, err := s.acquirePublicationLease(dst)
	if err != nil {
		return err
	}
	defer releasePublication()

	useSignedDB := s.signedDB()
	var gnupgDir *string

	// A package may be registered in several arches (an arch=any package is in
	// every configured arch); promote each arch the source tier lists it under.
	var promotedArches []string
	for _, arch := range s.repoArches(src) {
		if err := ctx.Err(); err != nil {
			return err
		}
		rr, err := s.pkgBinaryRepo.RemoteRepo(src, arch)
		if err != nil {
			if errors.Is(err, blob.ErrNotFound) {
				continue // this arch's tier db is empty
			}
			return errors.WrapErr(err, "read source tier db")
		}
		p := rr.PkgByPkgName(pkgname)
		if p == nil {
			continue
		}
		if version != "" && p.Version() != version {
			return fmt.Errorf("%w: %s in %s is %s, not the requested %s", domain.ErrInvalid, pkgname, src, p.Version(), version)
		}
		// An arch=any package's file lives once under "any/"; a concrete package
		// lives under its own arch.
		storeArch := arch
		if p.Arch() == "any" {
			storeArch = "any"
		}
		if err := s.promoteOneArch(src, dst, arch, storeArch, p.Path(), p.Name(), p.Version(), useSignedDB, gnupgDir); err != nil {
			return errors.WrapErr(err, fmt.Sprintf("promote %s %s/%s", pkgname, dst, arch))
		}
		promotedArches = append(promotedArches, arch)
	}
	if len(promotedArches) == 0 {
		return fmt.Errorf("%w: %s not found in %s", domain.ErrNotFound, pkgname, src)
	}

	if !rc.PromotionKeepsSource() {
		s.dropFromSource(src, pkgname, promotedArches, useSignedDB, gnupgDir)
	}
	return nil
}

// promoteOneArch copies the source tier's package into the destination tier and
// registers it there, so the promoted tier serves the same built bytes.
func (s *Service) promoteOneArch(src, dst, arch, storeArch, filename, pkgname, version string, useSignedDB bool, gnupgDir *string) error {
	current, exists, err := s.publishedPackage(dst, arch, pkgname)
	if err != nil {
		return errors.WrapErr(err, "read target tier package state")
	}
	if exists {
		cmp, _ := alpm.VerCmp(version, current.version)
		if cmp < 0 {
			return fmt.Errorf("%w: target tier already has newer %s %s", domain.ErrConflict, pkgname, current.version)
		}
		if cmp == 0 && current.fileName != filename {
			return fmt.Errorf("%w: target tier has %s %s under a different immutable filename", domain.ErrConflict, pkgname, version)
		}
	}
	pkgSeek, cleanup, err := s.spoolTierFile(src, storeArch, filename)
	if err != nil {
		return err
	}
	defer cleanup()

	if _, err := s.pkgBinaryRepo.StoreFileImmutable(dst, storeArch, pkgSeek); err != nil {
		return errors.WrapErr(err, "store package pointer in target tier")
	}

	// Carry the detached signature across too when the source tier has one.
	sigName := filename + ".sig"
	if sigSeek, sigCleanup, serr := s.spoolTierFile(src, storeArch, sigName); serr == nil {
		defer sigCleanup()
		named := stream.NewFileStream(sigName, sigSeek.ContentType(), sigSeek)
		if _, err := s.pkgBinaryRepo.StoreFileImmutable(dst, storeArch, named); err != nil {
			return errors.WrapErr(err, "store signature pointer in target tier")
		}
	} else if !errors.Is(serr, blob.ErrNotFound) {
		return errors.WrapErr(serr, "read source tier signature")
	}

	if _, err := pkgSeek.Seek(0, io.SeekStart); err != nil {
		return errors.WrapErr(err, "rewind package for registration")
	}
	item := repository.RepoAddItem{
		Pkg:             pkgSeek,
		CheckCurrent:    true,
		ExpectedName:    pkgname,
		IntendedVersion: version,
		IntendedFile:    filename,
	}
	if exists {
		item.ExpectedCurrentVersion = current.version
		item.ExpectedCurrentFile = current.fileName
	}
	if err := s.pkgBinaryRepo.RepoAddBatch(dst, arch, []repository.RepoAddItem{item}, useSignedDB, gnupgDir); err != nil {
		return errors.WrapErr(err, "register package in target tier db")
	}
	if err := s.pkgNameRepo.StorePackageFile(dst, storeArch, pkgname, filename); err != nil {
		return errors.WrapErr(err, "record promoted package file name")
	}
	return nil
}

// dropFromSource removes the package from the source tier under the move policy.
// Best-effort: promotion already succeeded, so a failed cleanup is logged rather
// than failing the whole promotion.
func (s *Service) dropFromSource(src, pkgname string, arches []string, useSignedDB bool, gnupgDir *string) {
	if err := s.RemovePkg(src, "", pkgname); err == nil {
		return
	}
	// RemovePkg leans on the name store; fall back to a direct per-arch de-register
	// so a move still clears the source tier's databases.
	for _, arch := range arches {
		if err := s.pkgBinaryRepo.RepoRemove(src, arch, pkgname, useSignedDB, gnupgDir); err != nil {
			slog.Warn("promote move: de-register from source tier", "pkg", pkgname, "repo", src, "arch", arch, "err", err)
		}
	}
}

// spoolTierFile copies a tier's stored file into a temp file and returns a
// re-seekable handle plus its cleanup. Promotion both re-stores and re-registers
// the bytes (each rewinds the stream), so a disk-backed temp keeps a large
// package off the heap.
func (s *Service) spoolTierFile(repo, arch, filename string) (stream.SeekFile, func(), error) {
	f, err := s.pkgBinaryRepo.FetchFile(repo, arch, filename)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	tmp, err := os.CreateTemp("", "ayato-promote-")
	if err != nil {
		return nil, nil, err
	}
	remove := func() { _ = tmp.Close(); _ = os.Remove(tmp.Name()) }
	if _, err := io.Copy(tmp, f); err != nil {
		remove()
		return nil, nil, errors.WrapErr(err, "spool tier file")
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		remove()
		return nil, nil, err
	}
	return stream.NewFileStream(path.Base(filename), f.ContentType(), noRemoveClose{tmp}), remove, nil
}

// noRemoveClose leaves file removal to the caller's cleanup, so a Close from an
// intermediate consumer (StoreFile/RepoAddBatch never close, but be defensive)
// cannot delete the temp before the last read.
type noRemoveClose struct{ *os.File }

func (noRemoveClose) Close() error { return nil }
