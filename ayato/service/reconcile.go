package service

import (
	"log/slog"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	pacmanpkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// OrphanObject is an unreferenced package object.
type OrphanObject struct {
	Arch string
	Name string
	Age  time.Duration
}

// ReconcileOrphans reports or deletes unreferenced package objects.
func (s *Service) ReconcileOrphans(repo string, olderThan time.Duration, dryRun bool) ([]OrphanObject, error) {
	repo = s.publishTarget(repo)
	now := time.Now()
	cutoff := now.Add(-olderThan)
	if olderThan < 0 {
		return nil, errors.New("orphan age must not be negative")
	}
	releasePublication, err := s.acquirePublicationLease(repo)
	if err != nil {
		return nil, err
	}
	defer releasePublication()

	arches, err := s.pkgBinaryRepo.Arches(repo)
	if err != nil {
		return nil, errors.WrapErr(err, "list arches for reconcile")
	}
	anyReferenced := make(map[string]struct{})
	for _, name := range dbArtifacts(repo) {
		anyReferenced[name] = struct{}{}
	}

	var orphans []OrphanObject
	for _, arch := range arches {
		referenced, err := s.referencedObjects(repo, arch)
		if err != nil {
			return nil, err
		}
		for name := range referenced {
			if pacmanpkg.IsAny(name) {
				anyReferenced[name] = struct{}{}
			}
		}
		found, err := s.reconcileDir(repo, arch, referenced, olderThan, dryRun, now, cutoff)
		if err != nil {
			return nil, err
		}
		orphans = append(orphans, found...)
	}

	// Scan the shared directory last, after collecting every arch=any filename
	// referenced by a concrete repository database.
	anyOrphans, err := s.reconcileDir(repo, "any", anyReferenced, olderThan, dryRun, now, cutoff)
	if err != nil {
		return nil, err
	}
	orphans = append(orphans, anyOrphans...)
	return orphans, nil
}

func (s *Service) reconcileDir(
	repo, arch string,
	referenced map[string]struct{},
	olderThan time.Duration,
	dryRun bool,
	now, cutoff time.Time,
) ([]OrphanObject, error) {
	infos, err := s.pkgBinaryRepo.FilesWithMeta(repo, arch)
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return nil, nil
		}
		return nil, errors.WrapErr(err, "list objects for reconcile")
	}
	var orphans []OrphanObject
	for _, info := range infos {
		if _, ok := referenced[info.Name]; ok {
			continue
		}
		if !pacmanpkg.IsArtifact(info.Name) {
			continue
		}
		if info.LastModified.IsZero() {
			slog.Warn("skip orphan with unknown modification time", "repo", repo, "arch", arch, "name", info.Name)
			continue
		}
		age := now.Sub(info.LastModified)
		if age < olderThan {
			continue
		}
		orphans = append(orphans, OrphanObject{Arch: arch, Name: info.Name, Age: age})
		if dryRun {
			slog.Info("orphan object (dry-run)", "repo", repo, "arch", arch, "name", info.Name, "age", age)
			continue
		}
		deleted, err := s.pkgBinaryRepo.DeleteOrphanIfUnchanged(repo, arch, info, cutoff)
		if errors.Is(err, blob.ErrSafeDeleteUnsupported) {
			slog.Warn("safe online orphan deletion unsupported; object only reported", "repo", repo, "arch", arch, "name", info.Name)
			continue
		}
		if err != nil {
			slog.Warn("failed to delete orphan object", "repo", repo, "arch", arch, "name", info.Name, "err", err)
			continue
		}
		if !deleted {
			slog.Info("kept orphan object renewed or changed concurrently", "repo", repo, "arch", arch, "name", info.Name)
			continue
		}
		slog.Info("deleted orphan object", "repo", repo, "arch", arch, "name", info.Name, "age", age)
	}
	return orphans, nil
}

// referencedObjects is the set of object names the reconcile must never delete
// for one (repo, arch): every registered package %FILENAME% and its .sig, plus
// the repo-DB artifacts (<repo>.db / .files, their .tar.gz, and the .sig of each).
func (s *Service) referencedObjects(repo, arch string) (map[string]struct{}, error) {
	referenced := make(map[string]struct{})
	for _, name := range dbArtifacts(repo) {
		referenced[name] = struct{}{}
	}

	rr, err := s.pkgBinaryRepo.RemoteRepo(repo, arch)
	if err != nil {
		// A missing db means nothing is registered yet; every package object under
		// the arch is then unreferenced. Surface a real backend error instead, so a
		// hiccup never GCs a repo whose db just failed to read.
		if errors.Is(err, blob.ErrNotFound) {
			return referenced, nil
		}
		return nil, errors.WrapErr(err, "read repo db for reconcile")
	}
	for _, p := range rr.Pkgs {
		fn := p.Path()
		if fn == "" {
			continue
		}
		referenced[fn] = struct{}{}
		referenced[fn+".sig"] = struct{}{}
	}
	return referenced, nil
}

// dbArtifacts lists the repo-DB objects (and their signatures) that are never
// package residue, so the reconcile always protects them.
func dbArtifacts(repo string) []string {
	return pacmanrepo.Artifacts(repo).All()
}
