package service

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

func (p *uploadPublication) storeObjects() error {
	for i := range p.uploads {
		upload := &p.uploads[i]
		if err := stream.Rewind(upload.pkgStream); err != nil {
			return errors.WrapErr(err, "failed to seek package file")
		}
		if _, err := p.service.pkgBinaryRepo.StoreFileImmutable(
			p.repo,
			upload.storeArch,
			upload.pkgStream,
		); err != nil {
			if errors.Is(err, repository.ErrImmutableObjectConflict) {
				return fmt.Errorf(
					"%w: package object %q already exists with different content",
					domain.ErrConflict,
					upload.storedName,
				)
			}
			return errors.WrapErr(err, "failed to store package")
		}
		if err := p.storeSignature(upload); err != nil {
			return err
		}
	}
	return nil
}

func (p *uploadPublication) storeSignature(upload *preparedUpload) error {
	if upload.sigStream == nil {
		return nil
	}
	if err := stream.Rewind(upload.sigStream); err != nil {
		return errors.WrapErr(err, "failed to seek signature file")
	}
	named := stream.NewFileStream(
		upload.sigName,
		upload.sigStream.ContentType(),
		upload.sigStream,
	)
	if _, err := p.service.pkgBinaryRepo.StoreFileImmutable(
		p.repo,
		upload.storeArch,
		named,
	); err != nil {
		if errors.Is(err, repository.ErrImmutableObjectConflict) {
			return fmt.Errorf(
				"%w: signature object %q already exists with different content",
				domain.ErrConflict,
				upload.sigName,
			)
		}
		return errors.WrapErr(err, "failed to store signature")
	}
	return nil
}

// buildArchBatches stores upload indexes instead of copying preparedUpload
// values, so rollback and commit always observe one state object.
func (p *uploadPublication) buildArchBatches() {
	p.byArch = make(map[string][]int)
	for index, upload := range p.uploads {
		for _, arch := range upload.dbArches {
			if _, exists := p.byArch[arch]; !exists {
				p.archOrder = append(p.archOrder, arch)
			}
			p.byArch[arch] = append(p.byArch[arch], index)
		}
	}
}

func (p *uploadPublication) commitDatabases() error {
	for _, arch := range p.archOrder {
		items := make([]repository.RepoAddItem, 0, len(p.byArch[arch]))
		for _, index := range p.byArch[arch] {
			items = append(items, p.uploads[index].repoAddItem(arch))
		}
		err := p.service.pkgBinaryRepo.RepoAddBatch(
			p.repo,
			arch,
			items,
			p.useSignedDB,
			nil,
		)
		if err == nil {
			p.rollback.committedArches = append(p.rollback.committedArches, arch)
			continue
		}
		if repository.CanonicalCommitted(err) {
			p.rollback.committedArches = append(p.rollback.committedArches, arch)
			p.rollback.needsReconcile[arch] = true
		}
		if errors.Is(err, repository.ErrPackageChanged) {
			return fmt.Errorf("%w: package changed during publish", domain.ErrConflict)
		}
		return errors.WrapErr(err, "failed to add to repo database")
	}
	return nil
}

func (p preparedUpload) repoAddItem(arch string) repository.RepoAddItem {
	item := repository.RepoAddItem{
		Pkg:             p.pkgStream,
		Sig:             p.sigStream,
		CheckCurrent:    true,
		ExpectedName:    p.pkgName,
		IntendedVersion: p.pkgVersion,
		IntendedFile:    p.storedName,
	}
	if old, exists := p.oldByArch[arch]; exists {
		item.ExpectedCurrentVersion = old.version
		item.ExpectedCurrentFile = old.fileName
	}
	return item
}
