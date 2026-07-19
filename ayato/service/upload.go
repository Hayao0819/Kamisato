package service

import (
	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

// preparedUpload is one validated package ready for publication. An arch=any
// package is stored once in any/ and registered in every concrete dbArch.
type preparedUpload struct {
	pkgStream  stream.SeekFile
	sigStream  stream.SeekFile
	pkgName    string
	pkgVersion string
	storeArch  string
	dbArches   []string
	storedName string
	sigName    string
	oldByArch  map[string]publishedPackage
}

// publishedPackage is the repository snapshot needed by the version gate and
// compensating transaction.
type publishedPackage struct {
	version   string
	fileName  string
	storeArch string
}

type archKey struct {
	arch string
	key  string
}

// uploadPublication coordinates one batch across validation, immutable object
// storage, per-arch database commits, and the package-name index.
type uploadPublication struct {
	service     *Service
	repo        string
	files       []*domain.UploadFiles
	useSignedDB bool
	uploads     []preparedUpload
	byArch      map[string][]int
	archOrder   []string
	rollback    *publicationRollback
}

func newUploadPublication(
	service *Service,
	repo string,
	files []*domain.UploadFiles,
) *uploadPublication {
	return &uploadPublication{
		service:     service,
		repo:        service.publishTarget(repo),
		files:       files,
		useSignedDB: service.signedDB(),
	}
}

// UploadFile publishes a single package.
func (s *Service) UploadFile(repo string, files *domain.UploadFiles) error {
	return s.UploadFiles(repo, []*domain.UploadFiles{files})
}

// UploadFiles publishes a batch with one atomic database update per arch.
func (s *Service) UploadFiles(repo string, files []*domain.UploadFiles) error {
	if len(files) == 0 {
		return nil
	}
	return newUploadPublication(s, repo, files).run()
}

func (p *uploadPublication) run() error {
	if err := p.ensureRepository(); err != nil {
		return err
	}
	if err := p.validateInputs(); err != nil {
		return err
	}

	release, err := p.service.acquirePublicationLease(p.repo)
	if err != nil {
		return err
	}
	defer release()

	if err := p.resolveRepositoryState(); err != nil {
		return err
	}
	p.buildArchBatches()

	rollback := newPublicationRollback(p)
	if err := rollback.capture(); err != nil {
		rollback.close()
		return err
	}
	defer rollback.close()
	p.rollback = rollback

	if err := p.storeObjects(); err != nil {
		rollback.run()
		return err
	}
	if err := p.commitDatabases(); err != nil {
		rollback.run()
		return err
	}
	if err := p.storePackageNames(); err != nil {
		rollback.run()
		return err
	}

	p.service.rebuildMergedIfUpstream(p.repo, p.archOrder)
	return nil
}
