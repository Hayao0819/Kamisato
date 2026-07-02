package service

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"strings"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/pkg/pacman/alpm"
	"github.com/Hayao0819/Kamisato/pkg/pacman/gpg"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

// preparedUpload is one validated package ready to be stored and registered.
// storeArch is where the file physically lives (an arch=any package is stored
// once under "any/" and shared via FetchFile's fallback); dbArches are the arches
// whose database registers it.
type preparedUpload struct {
	pkgStream  stream.SeekFile
	sigStream  stream.SeekFile // nil when no signature
	pkgName    string
	storeArch  string
	dbArches   []string
	storedName string
	sigName    string
}

// prepareUpload validates and verifies one package without storing anything: it
// reads the metadata, gates the signature (a present signature is always
// verified; a missing one is allowed only when RequireSign is false), and
// resolves the storage arch and the db arches. Storing nothing here lets a bad
// package in a batch fail the whole publish before any state changes. kr is the
// verification keyring built once per batch; nil means no trust root.
func (s *Service) prepareUpload(repo string, files *domain.UploadFiles, kr *gpg.Keyring) (preparedUpload, error) {
	pkgFileStream := files.PkgFile
	p, err := pkg.ReadBinaryPackage(pkgFileStream.FileName(), pkgFileStream)
	if err != nil {
		return preparedUpload{}, fmt.Errorf("%w: failed to read package from binary: %w", domain.ErrInvalidUpload, err)
	}
	pi := p.PKGINFO()
	slog.Info("get pkg from bin", "pkgname", pi.PkgName, "pkgver", pi.PkgVer)

	if s.cfg != nil && s.cfg.RequireBuildinfoProvenance {
		if err := s.checkBuildinfoProvenance(pkgFileStream); err != nil {
			return preparedUpload{}, err
		}
	}

	if err := s.checkProtectedNames(pi); err != nil {
		return preparedUpload{}, err
	}

	hasSig := files.SigFile != nil
	if s.cfg != nil && s.cfg.RequireSign && !hasSig {
		return preparedUpload{}, fmt.Errorf("%w: package signature is required but none was provided", domain.ErrInvalidUpload)
	}
	if hasSig {
		if kr == nil {
			// A signature is present but there is no trust root to verify it;
			// reject rather than store an unvalidated signature.
			return preparedUpload{}, fmt.Errorf("package signature present but no trust root (verify.keyring or a registered signer) is configured to validate it")
		}
		if _, err := pkgFileStream.Seek(0, io.SeekStart); err != nil {
			return preparedUpload{}, errwrap.WrapErr(err, "failed to seek package file for verification")
		}
		if _, err := files.SigFile.Seek(0, io.SeekStart); err != nil {
			return preparedUpload{}, errwrap.WrapErr(err, "failed to seek signature file for verification")
		}
		fpr, verr := kr.VerifyDetached(pkgFileStream, files.SigFile)
		if verr != nil {
			return preparedUpload{}, fmt.Errorf("%w: package signature verification failed: %s", domain.ErrInvalidUpload, verr.Error())
		}
		slog.Info("package signature verified", "pkgname", pi.PkgName, "fingerprint", fpr)
	}

	// pi.Arch comes from attacker-controlled .PKGINFO; reject anything that is not
	// a single safe path component so it cannot escape the repo dir as a storage
	// subdirectory.
	if pi.Arch == "" || strings.ContainsRune(pi.Arch, '/') || strings.Contains(pi.Arch, "..") {
		return preparedUpload{}, fmt.Errorf("%w: invalid package arch %q", domain.ErrInvalidUpload, pi.Arch)
	}
	dbArches, err := s.targetArches(repo, pi.Arch)
	if err != nil {
		return preparedUpload{}, err
	}

	// Fail-closed version gate: reject a downgrade or a re-publish of an already
	// present version. Running here, in the up-front validation loop, means a bad
	// package anywhere in a batch aborts the whole publish before anything is
	// stored. Wrapping ErrInvalidUpload maps it to HTTP 400.
	for _, a := range dbArches {
		cur, ok, verr := s.publishedVersion(repo, a, pi.PkgName)
		if verr != nil {
			return preparedUpload{}, verr
		}
		if !ok {
			continue
		}
		cmp, _ := alpm.VerCmp(pi.PkgVer, cur)
		if cmp < 0 {
			return preparedUpload{}, fmt.Errorf("%w: %s %s is older than the published %s (downgrade rejected)", domain.ErrInvalidUpload, pi.PkgName, pi.PkgVer, cur)
		}
		if cmp == 0 {
			return preparedUpload{}, fmt.Errorf("%w: %s %s is already published (duplicate rejected)", domain.ErrInvalidUpload, pi.PkgName, pi.PkgVer)
		}
	}

	storedName := path.Base(pkgFileStream.FileName())
	prep := preparedUpload{
		pkgStream:  pkgFileStream,
		pkgName:    pi.PkgName,
		storeArch:  pi.Arch,
		dbArches:   dbArches,
		storedName: storedName,
		sigName:    storedName + ".sig",
	}
	if hasSig {
		prep.sigStream = files.SigFile
	}
	return prep, nil
}

// checkBuildinfoProvenance is a fail-closed ingest gate: it rejects a package
// whose .BUILDINFO builddir is not the expected sandbox root — a signal it was not
// built in miko's clean environment (builder infection) — and a package with no
// .BUILDINFO at all. It leaves the stream re-seekable for the caller.
func (s *Service) checkBuildinfoProvenance(pkgFileStream stream.SeekFile) error {
	if _, err := pkgFileStream.Seek(0, io.SeekStart); err != nil {
		return errwrap.WrapErr(err, "failed to seek package file for buildinfo check")
	}
	bi, err := pkg.ReadBuildInfo(pkgFileStream)
	if err != nil {
		if errors.Is(err, pkg.ErrBuildInfoNotFound) {
			return fmt.Errorf("%w: package has no .BUILDINFO but provenance is required", domain.ErrInvalidUpload)
		}
		return fmt.Errorf("%w: failed to read package .BUILDINFO: %s", domain.ErrInvalidUpload, err.Error())
	}
	want := s.cfg.ExpectedBuildDir()
	if bi.BuildDir != want {
		return fmt.Errorf("%w: package builddir %q is not the expected sandbox root %q (not built in the clean environment)", domain.ErrInvalidUpload, bi.BuildDir, want)
	}
	return nil
}

// checkProtectedNames rejects an upload that would shadow an official package
// name: a match on the pkgname, or on the bare name of any provides/replaces
// entry, or on any group, is a supply-chain masquerade (e.g. a package claiming to
// provide "glibc"). Off unless conf.ProtectedNames is set.
func (s *Service) checkProtectedNames(pi *raiou.PKGINFO) error {
	if s.cfg == nil || len(s.cfg.ProtectedNames) == 0 {
		return nil
	}
	protected := make(map[string]struct{}, len(s.cfg.ProtectedNames))
	for _, n := range s.cfg.ProtectedNames {
		protected[n] = struct{}{}
	}
	candidates := make([]string, 0, 1+len(pi.Provides)+len(pi.Replaces)+len(pi.Group))
	candidates = append(candidates, pi.PkgName)
	for _, p := range pi.Provides {
		candidates = append(candidates, depName(p))
	}
	for _, r := range pi.Replaces {
		candidates = append(candidates, depName(r))
	}
	candidates = append(candidates, pi.Group...)
	for _, c := range candidates {
		if _, ok := protected[c]; ok {
			return fmt.Errorf("%w: package %q collides with protected official name %q", domain.ErrConflict, pi.PkgName, c)
		}
	}
	return nil
}

// depName strips a version constraint from a provides/replaces entry (foo=1.2,
// foo>=1.0), leaving the bare name the protected-name match is done on.
func depName(entry string) string {
	if i := strings.IndexAny(entry, "=<>"); i >= 0 {
		return entry[:i]
	}
	return entry
}

// UploadFile publishes a single package; it is the one-item form of UploadFiles.
func (s *Service) UploadFile(repo string, files *domain.UploadFiles) error {
	return s.UploadFiles(repo, []*domain.UploadFiles{files})
}

// UploadFiles publishes one or more packages atomically. It validates and
// verifies every package first, stores each file, then registers them all in
// each affected (repo, arch) database with a single RepoAddBatch per arch — so a
// multi-package push (a split package, or a rebuild set) lands as one atomic
// database update rather than N partial ones. On any error it rolls back every
// stored file and database entry.
func (s *Service) UploadFiles(repo string, files []*domain.UploadFiles) error {
	if len(files) == 0 {
		return nil
	}
	// A tiered repo publishes into its staging tier; testing and stable are reached
	// only by an explicit promotion, keeping building and publishing separate.
	repo = s.publishTarget(repo)
	useSignedDB := s.signedDB()
	// gnupgDir is only the CLI tool's GNUPGHOME; native signing takes its key from
	// the environment (AYATO_DB_SIGNING_KEY), wired into the repository layer at
	// startup, so it stays nil.
	var gnupgDir *string

	if s.pkgBinaryRepo.VerifyPkgRepo(repo) != nil {
		slog.Warn("repository directory not found", "repo", repo)
		if err := s.initRepo(repo, useSignedDB, gnupgDir); err != nil {
			return errwrap.WrapErr(err, "failed to initialize repo")
		}
	}

	// Build the verification keyring once for the whole batch: it is identical for
	// every package and rebuilding it per file re-runs the (KV-backed) signer
	// lookup N times. Skip it entirely when nothing in the batch is signed.
	var kr *gpg.Keyring
	for _, f := range files {
		if f.SigFile != nil {
			var kerr error
			if kr, kerr = s.verifyKeyring(); kerr != nil {
				return fmt.Errorf("build signature keyring err: %s", kerr.Error())
			}
			break
		}
	}

	// Validate and verify every package up front, so a bad package in the batch
	// fails the whole publish before anything is stored.
	preps := make([]preparedUpload, 0, len(files))
	for _, f := range files {
		slog.Info("upload pkg file", "file", f.PkgFile.FileName())
		prep, err := s.prepareUpload(repo, f, kr)
		if err != nil {
			return err
		}
		preps = append(preps, prep)
	}

	// Rollback state. archKey is (arch, name-or-pkgname) depending on the slice.
	type archKey struct{ arch, key string }
	var stored, added, named []archKey
	rollback := func() {
		for _, a := range added {
			if err := s.pkgBinaryRepo.RepoRemove(repo, a.arch, a.key, useSignedDB, gnupgDir); err != nil {
				slog.Warn("failed to roll back repo-add", "repo", repo, "arch", a.arch, "pkg", a.key, "err", err)
			}
		}
		for _, f := range stored {
			if err := s.pkgBinaryRepo.DeleteFile(repo, f.arch, f.key); err != nil {
				slog.Warn("failed to clean up stored file after upload error", "repo", repo, "arch", f.arch, "filename", f.key, "err", err)
			}
		}
		for _, n := range named {
			if err := s.pkgNameRepo.DeletePackageFileEntry(n.arch, n.key); err != nil {
				slog.Warn("failed to roll back package-name entry", "arch", n.arch, "pkg", n.key, "err", err)
			}
		}
	}

	// Store every package file (and its signature) under its arch.
	for _, p := range preps {
		if _, err := p.pkgStream.Seek(0, io.SeekStart); err != nil {
			rollback()
			return errwrap.WrapErr(err, "failed to seek package file")
		}
		if err := s.pkgBinaryRepo.StoreFile(repo, p.storeArch, p.pkgStream); err != nil {
			rollback()
			return errwrap.WrapErr(err, "failed to store file")
		}
		stored = append(stored, archKey{p.storeArch, p.storedName})
		if p.sigStream != nil {
			if _, err := p.sigStream.Seek(0, io.SeekStart); err != nil {
				rollback()
				return errwrap.WrapErr(err, "failed to seek signature file")
			}
			// StoreFile keys the on-disk name off FileName(), so re-wrap the sig
			// under "<storedName>.sig". Verification already rejected bad sigs.
			sigToStore := stream.NewFileStream(p.sigName, p.sigStream.ContentType(), p.sigStream)
			if err := s.pkgBinaryRepo.StoreFile(repo, p.storeArch, sigToStore); err != nil {
				rollback()
				return errwrap.WrapErr(err, "failed to store signature file")
			}
			stored = append(stored, archKey{p.storeArch, p.sigName})
		}
	}

	// Group packages by db arch; each arch's database is updated once, atomically.
	byArch := map[string][]repository.RepoAddItem{}
	pkgsByArch := map[string][]string{}
	var archOrder []string
	for _, p := range preps {
		for _, a := range p.dbArches {
			if _, ok := byArch[a]; !ok {
				archOrder = append(archOrder, a)
			}
			byArch[a] = append(byArch[a], repository.RepoAddItem{Pkg: p.pkgStream})
			pkgsByArch[a] = append(pkgsByArch[a], p.pkgName)
		}
	}
	for _, a := range archOrder {
		if err := s.pkgBinaryRepo.RepoAddBatch(repo, a, byArch[a], useSignedDB, gnupgDir); err != nil {
			rollback()
			return errwrap.WrapErr(err, "failed to add to repo database")
		}
		for _, pn := range pkgsByArch[a] {
			added = append(added, archKey{a, pn})
		}
	}

	// Record each package's file name.
	for _, p := range preps {
		if err := s.pkgNameRepo.StorePackageFile(p.storeArch, p.pkgName, p.storedName); err != nil {
			rollback()
			return errwrap.WrapErr(err, "failed to store package file name")
		}
		named = append(named, archKey{p.storeArch, p.pkgName})
	}
	return nil
}

// configuredArches returns the repo's concrete arches, dropping "" and "any"
// (pacman has no os/any database; an arch=any package is registered in each
// concrete arch instead).
func (s *Service) configuredArches(repo string) []string {
	rc := s.cfg.ResolveRepo(repo)
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

// signedDB reports whether uploads should produce a signed pacman database. The
// signing key itself is wired into the repository layer at startup.
func (s *Service) signedDB() bool {
	return s.cfg != nil && s.cfg.Sign.DB
}

// publishTarget maps a publish addressed at a tiered repo's logical name to its
// staging tier, so a build always lands in staging. A non-tiered repo, or a name
// already addressing a specific tier, is returned unchanged.
func (s *Service) publishTarget(repo string) string {
	if s.cfg == nil {
		return repo
	}
	if rc := s.cfg.Repo(repo); rc != nil && rc.Tiered {
		return rc.TierRepo(conf.TierStaging)
	}
	return repo
}

// publishedVersion returns the version of pkgname currently published in
// (repo, arch), reading the authoritative .db (not the NameStore cache, which can
// legitimately miss). A missing db or absent package is ("", false, nil); only a
// real backend error is surfaced, so the gate fails closed on an unreadable db.
func (s *Service) publishedVersion(repo, arch, pkgname string) (string, bool, error) {
	rr, err := s.pkgBinaryRepo.RemoteRepo(repo, arch)
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return "", false, nil
		}
		return "", false, errwrap.WrapErr(err, "read repo db for version gate")
	}
	p := rr.PkgByPkgName(pkgname)
	if p == nil {
		return "", false, nil
	}
	return p.Version(), true, nil
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
