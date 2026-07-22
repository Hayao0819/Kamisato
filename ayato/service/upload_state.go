package service

import (
	"fmt"
	"log/slog"
	"path"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	alpm "github.com/Hayao0819/dyalpm"
)

func (p *uploadPublication) ensureRepository() error {
	if p.service.pkgBinaryRepo.VerifyPkgRepo(p.repo) == nil {
		return nil
	}
	slog.Warn("repository directory not found", "repo", p.repo)
	if err := p.service.initRepo(p.repo, p.useSignedDB, nil); err != nil {
		return errors.WrapErr(err, "failed to initialize repo")
	}
	return nil
}

func (p *uploadPublication) resolveRepositoryState() error {
	for i := range p.uploads {
		upload := &p.uploads[i]
		if upload.storeArch != "any" &&
			!p.service.archAccepted(p.repo, upload.storeArch) {
			return fmt.Errorf(
				"%w: arch %q is not served by repo %q",
				domain.ErrInvalidUpload,
				upload.storeArch,
				p.repo,
			)
		}
	}
	if err := p.seedNewArches(); err != nil {
		return err
	}

	seenPackages := make(map[archKey]struct{})
	for i := range p.uploads {
		if err := p.resolveUploadState(&p.uploads[i], seenPackages); err != nil {
			return err
		}
	}
	return nil
}

// seedNewArches runs before resolving arch=any targets and version snapshots.
// Thus a concrete arch introduced by this batch participates in the same batch's
// any-package fan-out and conditional update.
func (p *uploadPublication) seedNewArches() error {
	if !p.service.catalog.AllowsNewArch(p.repo) {
		return nil
	}
	stored := make(map[string]struct{})
	for _, arch := range p.service.storedArches(p.repo) {
		stored[arch] = struct{}{}
	}
	for _, upload := range p.uploads {
		if upload.storeArch == "any" {
			continue
		}
		if _, exists := stored[upload.storeArch]; exists {
			continue
		}
		if err := p.service.ensureArchSeeded(
			p.repo,
			upload.storeArch,
			p.useSignedDB,
			nil,
		); err != nil {
			return errors.WrapErr(err, "failed to seed new arch")
		}
		stored[upload.storeArch] = struct{}{}
	}
	return nil
}

func (p *uploadPublication) resolveUploadState(
	upload *preparedUpload,
	seenPackages map[archKey]struct{},
) error {
	arches, err := p.service.targetArches(p.repo, upload.storeArch)
	if err != nil {
		return err
	}
	upload.dbArches = make([]string, 0, len(arches))
	upload.oldByArch = make(map[string]publishedPackage)
	for _, arch := range arches {
		key := archKey{arch: arch, key: upload.pkgName}
		if _, duplicate := seenPackages[key]; duplicate {
			return fmt.Errorf(
				"%w: package %q targets %s more than once in one batch",
				domain.ErrInvalidUpload,
				upload.pkgName,
				arch,
			)
		}
		seenPackages[key] = struct{}{}

		current, exists, err := p.service.publishedPackage(p.repo, arch, upload.pkgName)
		if err != nil {
			return err
		}
		if !exists {
			upload.dbArches = append(upload.dbArches, arch)
			continue
		}
		comparison := alpm.VerCmp(upload.pkgVersion, current.version)
		switch {
		case comparison < 0:
			return fmt.Errorf(
				"%w: %s %s is older than the published %s",
				domain.ErrInvalidUpload,
				upload.pkgName,
				upload.pkgVersion,
				current.version,
			)
		case comparison == 0:
			// A same-version re-upload is a no-op, not an error: parallel CI
			// arch jobs each rebuild an arch=any package and race to publish
			// it. The first stored artifact stays authoritative.
			slog.Info("skipping already-published package",
				"repo", p.repo, "arch", arch, "pkg", upload.pkgName, "version", upload.pkgVersion)
		default:
			upload.oldByArch[arch] = current
			upload.dbArches = append(upload.dbArches, arch)
		}
	}
	return nil
}

func (s *Service) publishedPackage(
	repo, arch, pkgName string,
) (publishedPackage, bool, error) {
	current, err := s.overlayRepo(repo, arch)
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return publishedPackage{}, false, nil
		}
		return publishedPackage{}, false, errors.WrapErr(err, "read repo db for version gate")
	}
	pkg := current.PkgByPkgName(pkgName)
	if pkg == nil {
		return publishedPackage{}, false, nil
	}
	fileName := path.Base(pkg.Path())
	storeArch := pkg.Arch()
	if storeArch != "any" {
		storeArch = arch
	}
	if fileName == "" || fileName == "." || storeArch == "" {
		return publishedPackage{}, false, fmt.Errorf(
			"published package %q in %s/%s has invalid storage identity",
			pkgName,
			repo,
			arch,
		)
	}
	return publishedPackage{
		version:   pkg.Version(),
		fileName:  fileName,
		storeArch: storeArch,
	}, true, nil
}

func (s *Service) signedDB() bool {
	return s.cfg != nil && s.cfg.Sign.DB
}

func (s *Service) publishTarget(repo string) string {
	if target, ok := s.catalog.PublishTarget(repo); ok {
		return target
	}
	return repo
}
