package service

import (
	"fmt"
	"io"
	"log/slog"
	"path"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/pkg/pacman/alpm"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
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
func (s *Service) prepareUpload(repo string, files *domain.UploadFiles, kr *sign.Keyring) (preparedUpload, error) {
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
			return preparedUpload{}, errors.WrapErr(err, "failed to seek package file for verification")
		}
		if _, err := files.SigFile.Seek(0, io.SeekStart); err != nil {
			return preparedUpload{}, errors.WrapErr(err, "failed to seek signature file for verification")
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
	// Refuse an upload that would add an architecture the repo does not serve, so a
	// mislabeled .PKGINFO arch cannot silently create a new arch dir. An arch=any
	// package never creates an arch (it fans out to the existing ones), so it is exempt.
	if pi.Arch != "any" && !s.archAccepted(repo, pi.Arch) {
		return preparedUpload{}, fmt.Errorf("%w: arch %q is not served by repo %q; add it to the repo's arches or set allow_new_arch", domain.ErrInvalidUpload, pi.Arch, repo)
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
		return errors.WrapErr(err, "failed to seek package file for buildinfo check")
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
			return errors.WrapErr(err, "failed to initialize repo")
		}
	}

	// Build the verification keyring once for the whole batch: it is identical for
	// every package and rebuilding it per file re-runs the (KV-backed) signer
	// lookup N times. Skip it entirely when nothing in the batch is signed.
	var kr *sign.Keyring
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

	// When the repo opts into new arches, a concrete upload may introduce one. Seed it
	// (backfilling the repo's existing arch=any packages) before storing, then
	// recompute each arch=any package's target set so it also lands in the just-added
	// arch. A repo that does not opt in never grows an arch here (prepareUpload gated
	// it), so this whole step — and its extra storage listing — is skipped.
	if s.cfg != nil && s.cfg.AllowsNewArch(repo) {
		preStored := make(map[string]struct{})
		for _, a := range s.storedArches(repo) {
			preStored[a] = struct{}{}
		}
		seededNew := false
		for _, p := range preps {
			if p.storeArch == "any" {
				continue
			}
			if _, ok := preStored[p.storeArch]; ok {
				continue
			}
			if err := s.ensureArchSeeded(repo, p.storeArch, useSignedDB, gnupgDir); err != nil {
				return errors.WrapErr(err, "failed to seed new arch")
			}
			preStored[p.storeArch] = struct{}{}
			seededNew = true
		}
		if seededNew {
			for i := range preps {
				if preps[i].storeArch == "any" {
					preps[i].dbArches = s.repoArches(repo)
				}
			}
		}
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
			if err := s.pkgNameRepo.DeletePackageFileEntry(repo, n.arch, n.key); err != nil {
				slog.Warn("failed to roll back package-name entry", "repo", repo, "arch", n.arch, "pkg", n.key, "err", err)
			}
		}
	}

	// Store every package file (and its signature) under its arch.
	for _, p := range preps {
		if _, err := p.pkgStream.Seek(0, io.SeekStart); err != nil {
			rollback()
			return errors.WrapErr(err, "failed to seek package file")
		}
		if err := s.pkgBinaryRepo.StoreFile(repo, p.storeArch, p.pkgStream); err != nil {
			rollback()
			return errors.WrapErr(err, "failed to store file")
		}
		stored = append(stored, archKey{p.storeArch, p.storedName})
		if p.sigStream != nil {
			if _, err := p.sigStream.Seek(0, io.SeekStart); err != nil {
				rollback()
				return errors.WrapErr(err, "failed to seek signature file")
			}
			// StoreFile keys the on-disk name off FileName(), so re-wrap the sig
			// under "<storedName>.sig". Verification already rejected bad sigs.
			sigToStore := stream.NewFileStream(p.sigName, p.sigStream.ContentType(), p.sigStream)
			if err := s.pkgBinaryRepo.StoreFile(repo, p.storeArch, sigToStore); err != nil {
				rollback()
				return errors.WrapErr(err, "failed to store signature file")
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
			byArch[a] = append(byArch[a], repository.RepoAddItem{Pkg: p.pkgStream, Sig: p.sigStream})
			pkgsByArch[a] = append(pkgsByArch[a], p.pkgName)
		}
	}
	for _, a := range archOrder {
		if err := s.pkgBinaryRepo.RepoAddBatch(repo, a, byArch[a], useSignedDB, gnupgDir); err != nil {
			rollback()
			return errors.WrapErr(err, "failed to add to repo database")
		}
		for _, pn := range pkgsByArch[a] {
			added = append(added, archKey{a, pn})
		}
	}

	// Record every package's file name in one batched write. Track the intended
	// keys up front: a bulk write is not per-key atomic, so on failure rollback
	// deletes the whole set (deleting a missing key is a no-op, and a stray entry
	// is anyway reconstructable from the .db).
	items := make([]repository.PackageFileEntry, 0, len(preps))
	for _, p := range preps {
		items = append(items, repository.PackageFileEntry{Arch: p.storeArch, Name: p.pkgName, FileName: p.storedName})
		named = append(named, archKey{p.storeArch, p.pkgName})
	}
	if err := s.pkgNameRepo.StorePackageFiles(repo, items); err != nil {
		rollback()
		return errors.WrapErr(err, "failed to store package file names")
	}
	// For an upstream-layered repo, refresh the served merged database so the newly
	// published packages appear in the merged view.
	s.rebuildMergedIfUpstream(repo, archOrder)
	return nil
}

// storedArches returns the arches the repo actually has package dirs for, derived
// from storage. The repository layer already excludes "any" (pacman has no os/any
// db; an arch=any package is registered in each concrete arch instead).
func (s *Service) storedArches(repo string) []string {
	arches, err := s.pkgBinaryRepo.Arches(repo)
	if err != nil {
		return nil
	}
	return arches
}

// declaredArches returns the arches the repo is configured to serve (its own, or the
// server default). Empty when nothing is configured.
func (s *Service) declaredArches(repo string) []string {
	if s.cfg == nil {
		return nil
	}
	return s.cfg.DeclaredArches(repo)
}

// repoArches is the set an arch=any package fans out to and that arch-wide
// operations iterate: the union of the configured (declared) arches and the arches
// already stored. Declared arches let a fresh repo accept an arch=any upload before
// any concrete package exists; stored arches keep serving an arch on disk but not
// (or no longer) declared.
func (s *Service) repoArches(repo string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, a := range append(s.declaredArches(repo), s.storedArches(repo)...) {
		if a == "" || a == "any" {
			continue
		}
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		out = append(out, a)
	}
	return out
}

// archAccepted reports whether a concrete-arch upload is allowed: the arch is one the
// repo already serves (declared or stored), or the repo opts into growing new arches.
// It stops a mislabeled package from silently adding an arch to an established repo.
func (s *Service) archAccepted(repo, arch string) bool {
	if s.cfg != nil && s.cfg.AllowsNewArch(repo) {
		return true
	}
	declared := s.declaredArches(repo)
	for _, a := range declared {
		if a == arch {
			return true // declared: accept without touching storage
		}
	}
	stored := s.storedArches(repo)
	// A repo with no arch set yet (declares none, stores none) has no baseline to
	// protect: its first upload establishes the set. Once the repo serves any arch, a
	// different one is a rejected increase unless allow_new_arch — so pushing x86_64
	// to an i686-only repo is refused.
	if len(declared) == 0 && len(stored) == 0 {
		return true
	}
	for _, a := range stored {
		if a == arch {
			return true
		}
	}
	return false
}

// ensureArchSeeded makes sure (repo, arch) has a database. When it creates one that
// did not exist, it backfills every arch=any package the repo already stores, so an
// arch added after those packages were published still serves them.
func (s *Service) ensureArchSeeded(repo, arch string, useSignedDB bool, gnupgDir *string) error {
	existed := false
	for _, a := range s.storedArches(repo) {
		if a == arch {
			existed = true
			break
		}
	}
	if err := s.pkgBinaryRepo.InitArch(repo, arch, useSignedDB, gnupgDir); err != nil {
		return err
	}
	// Backfill signatures for a db published before signing was enabled, so a repo
	// does not serve an unsigned db (which a required SigLevel rejects) until its next
	// mutate. Idempotent: a no-op once the db is signed.
	if useSignedDB {
		if err := s.pkgBinaryRepo.BackfillSignatures(repo, arch); err != nil {
			return err
		}
	}
	if existed {
		return nil
	}
	return s.backfillAnyInto(repo, arch, useSignedDB, gnupgDir)
}

// backfillAnyInto registers every arch=any package the repo already stores into a
// newly created arch's database (pacman has no os/any db; an any package must be in
// each arch's db to be installable). A no-op when the repo stores no any packages.
func (s *Service) backfillAnyInto(repo, arch string, useSignedDB bool, gnupgDir *string) error {
	files, err := s.pkgBinaryRepo.Files(repo, "any")
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return nil
		}
		return errors.WrapErr(err, "list any packages for backfill")
	}
	var items []repository.RepoAddItem
	var cleanups []func()
	defer func() {
		for _, c := range cleanups {
			c()
		}
	}()
	for _, f := range files {
		if !strings.Contains(f, ".pkg.tar.") || strings.HasSuffix(f, ".sig") {
			continue
		}
		pkgFile, cleanup, err := s.spoolTierFile(repo, "any", f)
		if err != nil {
			return errors.WrapErr(err, "spool any package for backfill")
		}
		cleanups = append(cleanups, cleanup)
		item := repository.RepoAddItem{Pkg: pkgFile}
		if sig, sigCleanup, serr := s.spoolTierFile(repo, "any", f+".sig"); serr == nil {
			cleanups = append(cleanups, sigCleanup)
			item.Sig = sig
		} else if !errors.Is(serr, blob.ErrNotFound) {
			return errors.WrapErr(serr, "spool any signature for backfill")
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return nil
	}
	slog.Info("backfilling arch=any packages into new arch", "repo", repo, "arch", arch, "count", len(items))
	return s.pkgBinaryRepo.RepoAddBatch(repo, arch, items, useSignedDB, gnupgDir)
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
	rr, err := s.overlayRepo(repo, arch)
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return "", false, nil
		}
		return "", false, errors.WrapErr(err, "read repo db for version gate")
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
	arches := s.repoArches(repo)
	if len(arches) == 0 {
		return nil, fmt.Errorf("repo %q has no architectures (declare the repo's arches or server default_arches) for an arch=any package", repo)
	}
	return arches, nil
}
