package service

import (
	"io"
	"log/slog"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/repository"
)

type publicationRollback struct {
	publication     *uploadPublication
	oldArtifacts    map[archKey]*spooledPackage
	oldNames        map[archKey]string
	committedArches []string
	needsReconcile  map[string]bool
	namesTouched    bool
}

func newPublicationRollback(publication *uploadPublication) *publicationRollback {
	return &publicationRollback{
		publication:    publication,
		oldArtifacts:   make(map[archKey]*spooledPackage),
		oldNames:       make(map[archKey]string),
		needsReconcile: make(map[string]bool),
	}
}

func (r *publicationRollback) capture() error {
	for _, upload := range r.publication.uploads {
		for _, old := range upload.oldByArch {
			artifactKey := archKey{arch: old.storeArch, key: old.fileName}
			nameKey := archKey{arch: old.storeArch, key: upload.pkgName}
			r.oldNames[nameKey] = old.fileName
			if _, exists := r.oldArtifacts[artifactKey]; exists {
				continue
			}
			artifact, err := r.publication.service.spoolPackage(
				r.publication.repo,
				old.storeArch,
				old.fileName,
			)
			if err != nil {
				return errors.WrapErr(err, "capture package for upload rollback")
			}
			r.oldArtifacts[artifactKey] = artifact
		}
	}
	return nil
}

func (r *publicationRollback) close() {
	for _, artifact := range r.oldArtifacts {
		artifact.close()
	}
}

func (r *publicationRollback) run() {
	protected := make(map[archKey]bool)
	for index := len(r.committedArches) - 1; index >= 0; index-- {
		arch := r.committedArches[index]
		r.restoreArch(arch, protected)
	}
	r.reconcileDatabases()
	r.restoreNames(protected)
}

func (r *publicationRollback) restoreArch(arch string, protected map[archKey]bool) {
	for _, uploadIndex := range r.publication.byArch[arch] {
		upload := &r.publication.uploads[uploadIndex]
		err := r.restorePackage(arch, upload)
		if repository.CanonicalCommitted(err) {
			r.needsReconcile[arch] = true
			err = nil
		}
		if err != nil {
			protected[archKey{arch: upload.storeArch, key: upload.pkgName}] = true
			slog.Error(
				"failed to restore repo database after upload error",
				"repo",
				r.publication.repo,
				"arch",
				arch,
				"pkg",
				upload.pkgName,
				"err",
				err,
			)
		}
	}
}

func (r *publicationRollback) restorePackage(
	arch string,
	upload *preparedUpload,
) error {
	old, existed := upload.oldByArch[arch]
	if !existed {
		return r.publication.service.pkgBinaryRepo.RepoRemoveIfMatch(
			r.publication.repo,
			arch,
			upload.pkgName,
			upload.pkgVersion,
			upload.storedName,
			r.publication.useSignedDB,
			nil,
		)
	}
	artifact := r.oldArtifacts[archKey{arch: old.storeArch, key: old.fileName}]
	if _, err := artifact.pkg.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err := r.publication.service.pkgBinaryRepo.StoreFileImmutable(
		r.publication.repo,
		old.storeArch,
		artifact.pkg,
	); err != nil {
		return err
	}
	if artifact.sig != nil {
		if _, err := artifact.sig.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if _, err := r.publication.service.pkgBinaryRepo.StoreFileImmutable(
			r.publication.repo,
			old.storeArch,
			artifact.sig,
		); err != nil {
			return err
		}
	}
	return r.publication.service.pkgBinaryRepo.RepoAddBatch(
		r.publication.repo,
		arch,
		[]repository.RepoAddItem{{
			Pkg:                    artifact.pkg,
			Sig:                    artifact.sig,
			CheckCurrent:           true,
			ExpectedName:           upload.pkgName,
			ExpectedCurrentVersion: upload.pkgVersion,
			ExpectedCurrentFile:    upload.storedName,
		}},
		r.publication.useSignedDB,
		nil,
	)
}

func (r *publicationRollback) reconcileDatabases() {
	for arch := range r.needsReconcile {
		if err := r.publication.service.pkgBinaryRepo.ReconcileDB(
			r.publication.repo,
			arch,
			r.publication.useSignedDB,
			nil,
		); err != nil {
			slog.Error(
				"failed to reconcile derived artifacts after upload rollback",
				"repo",
				r.publication.repo,
				"arch",
				arch,
				"err",
				err,
			)
		}
	}
}
