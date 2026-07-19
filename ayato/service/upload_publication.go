package service

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/platform"
	"github.com/Hayao0819/Kamisato/ayato/repository"
)

func (p *uploadPublication) storeObjects() error {
	for i := range p.uploads {
		upload := &p.uploads[i]
		if err := p.storeObject(
			upload.storeArch,
			"package",
			upload.storedName,
			upload.pkgStream,
		); err != nil {
			return err
		}
		if err := p.storeObject(
			upload.storeArch,
			"signature",
			upload.sigName,
			upload.sigStream,
		); err != nil {
			return err
		}
	}
	return nil
}

func (p *uploadPublication) storeObject(
	arch, kind, name string,
	file platform.SeekFile,
) error {
	if file == nil {
		return nil
	}
	if file.FileName() != name {
		file = platform.NewFileStream(name, file.ContentType(), file)
	}
	if err := p.service.storeImmutableFile(p.repo, arch, file); err != nil {
		if errors.Is(err, repository.ErrImmutableObjectConflict) {
			return fmt.Errorf(
				"%w: %s object %q already exists with different content",
				domain.ErrConflict,
				kind,
				name,
			)
		}
		return errors.WrapErr(err, "failed to store "+kind)
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
