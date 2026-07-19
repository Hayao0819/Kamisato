package service

import (
	"log/slog"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
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
			if strings.Contains(name, "-any.pkg.tar.") {
				anyReferenced[name] = struct{}{}
			}
		}

		infos, err := s.pkgBinaryRepo.FilesWithMeta(repo, arch)
		if err != nil {
			if errors.Is(err, blob.ErrNotFound) {
				continue
			}
			return nil, errors.WrapErr(err, "list objects for reconcile")
		}

		for _, info := range infos {
			if _, ok := referenced[info.Name]; ok {
				continue
			}
			if !isPackageArtifact(info.Name) {
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
			orphan := OrphanObject{Arch: arch, Name: info.Name, Age: age}
			orphans = append(orphans, orphan)
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
	}

	anyOrphans, err := s.reconcileAnyDir(repo, anyReferenced, olderThan, dryRun, now, cutoff)
	if err != nil {
		return nil, err
	}
	orphans = append(orphans, anyOrphans...)
	return orphans, nil
}

// reconcileAnyDir GCs orphans in the shared "any/" directory. It runs after the
// concrete arches so anyReferenced holds every registered arch=any filename; an
// object there that no concrete db references is residue of a presigned any
// upload PUT but never finalized.
func (s *Service) reconcileAnyDir(repo string, anyReferenced map[string]struct{}, olderThan time.Duration, dryRun bool, now, cutoff time.Time) ([]OrphanObject, error) {
	infos, err := s.pkgBinaryRepo.FilesWithMeta(repo, "any")
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return nil, nil
		}
		return nil, errors.WrapErr(err, "list any objects for reconcile")
	}
	var orphans []OrphanObject
	for _, info := range infos {
		if _, ok := anyReferenced[info.Name]; ok {
			continue
		}
		if !isPackageArtifact(info.Name) {
			continue
		}
		if info.LastModified.IsZero() {
			slog.Warn("skip orphan with unknown modification time", "repo", repo, "arch", "any", "name", info.Name)
			continue
		}
		age := now.Sub(info.LastModified)
		if age < olderThan {
			continue
		}
		orphans = append(orphans, OrphanObject{Arch: "any", Name: info.Name, Age: age})
		if dryRun {
			slog.Info("orphan object (dry-run)", "repo", repo, "arch", "any", "name", info.Name, "age", age)
			continue
		}
		deleted, err := s.pkgBinaryRepo.DeleteOrphanIfUnchanged(repo, "any", info, cutoff)
		if errors.Is(err, blob.ErrSafeDeleteUnsupported) {
			slog.Warn("safe online orphan deletion unsupported; object only reported", "repo", repo, "arch", "any", "name", info.Name)
			continue
		}
		if err != nil {
			slog.Warn("failed to delete orphan object", "repo", repo, "arch", "any", "name", info.Name, "err", err)
			continue
		}
		if !deleted {
			slog.Info("kept orphan object renewed or changed concurrently", "repo", repo, "arch", "any", "name", info.Name)
			continue
		}
		slog.Info("deleted orphan object", "repo", repo, "arch", "any", "name", info.Name, "age", age)
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
	bases := []string{
		repo + ".db",
		repo + ".db.tar.gz",
		repo + ".files",
		repo + ".files.tar.gz",
	}
	out := make([]string, 0, len(bases)*2)
	for _, b := range bases {
		out = append(out, b, b+".sig")
	}
	return out
}

// isPackageArtifact reports whether a name looks like a package or its detached
// signature (<...>.pkg.tar.<ext> optionally + .sig), so the reconcile only ever
// considers package residue and leaves any other stray object alone.
func isPackageArtifact(name string) bool {
	return strings.Contains(name, ".pkg.tar.")
}
