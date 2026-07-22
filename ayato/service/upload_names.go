package service

import (
	"log/slog"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/repository"
)

func (p *uploadPublication) storePackageNames() error {
	entries := make([]repository.PackageFileEntry, 0, len(p.uploads))
	for _, upload := range p.uploads {
		// Skipped as already published on every target arch; its name entry is
		// already in place.
		if len(upload.dbArches) == 0 {
			continue
		}
		entries = append(entries, repository.PackageFileEntry{
			Arch:     upload.storeArch,
			Name:     upload.pkgName,
			FileName: upload.storedName,
		})
	}
	if len(entries) == 0 {
		return nil
	}
	p.rollback.namesTouched = true
	if err := p.service.pkgNameRepo.StorePackageFiles(p.repo, entries); err != nil {
		return errors.WrapErr(err, "failed to store package file names")
	}

	newNames := make(map[archKey]struct{}, len(entries))
	for _, entry := range entries {
		newNames[archKey{arch: entry.Arch, key: entry.Name}] = struct{}{}
	}
	for key := range p.rollback.oldNames {
		if _, retained := newNames[key]; retained {
			continue
		}
		if err := p.service.pkgNameRepo.DeletePackageFileEntry(
			p.repo,
			key.arch,
			key.key,
		); err != nil {
			return errors.WrapErr(err, "failed to remove superseded package file name")
		}
	}
	return nil
}

func (r *publicationRollback) restoreNames(protected map[archKey]bool) {
	if !r.namesTouched {
		return
	}
	seen := make(map[archKey]struct{})
	var restoreKeys []archKey
	for _, upload := range r.publication.uploads {
		key := archKey{arch: upload.storeArch, key: upload.pkgName}
		if _, duplicate := seen[key]; duplicate || protected[key] {
			continue
		}
		seen[key] = struct{}{}
		current, err := r.publication.service.pkgNameRepo.PackageFile(
			r.publication.repo,
			key.arch,
			key.key,
		)
		if err != nil || current != upload.storedName {
			continue
		}
		if err := r.publication.service.pkgNameRepo.DeletePackageFileEntry(
			r.publication.repo,
			key.arch,
			key.key,
		); err != nil {
			slog.Warn("failed to remove new package-name entry", "pkg", key.key, "err", err)
			continue
		}
		restoreKeys = append(restoreKeys, key)
	}
	r.restoreOldNameEntries(restoreKeys)
}

func (r *publicationRollback) restoreOldNameEntries(keys []archKey) {
	entries := make([]repository.PackageFileEntry, 0, len(keys))
	for _, key := range keys {
		if fileName, exists := r.oldNames[key]; exists {
			entries = append(entries, repository.PackageFileEntry{
				Arch:     key.arch,
				Name:     key.key,
				FileName: fileName,
			})
		}
	}
	if err := r.publication.service.pkgNameRepo.StorePackageFiles(
		r.publication.repo,
		entries,
	); err != nil {
		slog.Error("failed to restore package-name entries", "err", err)
	}
}
