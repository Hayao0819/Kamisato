package service

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/platform"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
)

func (s *Service) acquirePublicationLease(repo string) (func(), error) {
	leaser, ok := s.pkgBinaryRepo.(interface {
		AcquirePublicationLease(string) (func(), error)
	})
	if !ok {
		return func() {}, nil
	}
	release, err := leaser.AcquirePublicationLease(repo)
	if err != nil {
		return nil, errors.WrapErr(err, "acquire repository publication lease")
	}
	return release, nil
}

// spooledPackage owns re-seekable package bytes and its optional signature.
// Publication, promotion, arch backfill, and rollback all need this same pair.
type spooledPackage struct {
	pkg      platform.SeekFile
	sig      platform.SeekFile
	cleanups []func()
}

func (s *Service) spoolPackage(
	repo, arch, filename string,
) (*spooledPackage, error) {
	pkgFile, cleanup, err := s.spoolRepositoryFile(repo, arch, filename)
	if err != nil {
		return nil, err
	}
	artifact := &spooledPackage{
		pkg:      pkgFile,
		cleanups: []func(){cleanup},
	}
	sigFile, sigCleanup, err := s.spoolRepositoryFile(repo, arch, filename+".sig")
	if err == nil {
		artifact.sig = sigFile
		artifact.cleanups = append(artifact.cleanups, sigCleanup)
		return artifact, nil
	}
	if errors.Is(err, blob.ErrNotFound) {
		return artifact, nil
	}
	artifact.close()
	return nil, errors.WrapErr(err, "spool package signature")
}

func (a *spooledPackage) close() {
	for _, cleanup := range a.cleanups {
		cleanup()
	}
}

func (s *Service) storeImmutableFile(
	repo, arch string,
	file platform.SeekFile,
) error {
	if file == nil {
		return nil
	}
	if err := platform.Rewind(file); err != nil {
		return errors.WrapErr(err, "rewind immutable object")
	}
	_, err := s.pkgBinaryRepo.StoreFileImmutable(repo, arch, file)
	return err
}

func (s *Service) storeSpooledPackage(
	repo, arch string,
	artifact *spooledPackage,
) error {
	for _, file := range []platform.SeekFile{artifact.pkg, artifact.sig} {
		if file == nil {
			continue
		}
		if err := s.storeImmutableFile(repo, arch, file); err != nil {
			return errors.WrapErr(err, "store package artifact "+file.FileName())
		}
	}
	return nil
}

func closeSpooledPackages(artifacts []*spooledPackage) {
	for _, artifact := range artifacts {
		artifact.close()
	}
}

func (s *Service) spoolRepositoryFile(
	repo, arch, filename string,
) (platform.SeekFile, func(), error) {
	source, err := s.pkgBinaryRepo.FetchFile(repo, arch, filename)
	if err != nil {
		return nil, nil, err
	}
	return spoolSource(source, filename, 0)
}

// spoolSource copies source to a temp file and closes source. maxBytes > 0
// bounds the copy so an oversized storage object cannot exhaust local disk.
func spoolSource(source platform.File, filename string, maxBytes int64) (platform.SeekFile, func(), error) {
	defer source.Close()

	tmp, err := os.CreateTemp("", "ayato-publication-")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}
	var reader io.Reader = source
	if maxBytes > 0 {
		reader = io.LimitReader(source, maxBytes+1)
	}
	written, err := io.Copy(tmp, reader)
	if err != nil {
		cleanup()
		return nil, nil, errors.WrapErr(err, "spool file")
	}
	if maxBytes > 0 && written > maxBytes {
		cleanup()
		return nil, nil, fmt.Errorf("%w: %s exceeds its size limit", domain.ErrInvalidUpload, filename)
	}
	if err := platform.Rewind(tmp); err != nil {
		cleanup()
		return nil, nil, err
	}
	file := platform.NewFileStream(
		path.Base(filename),
		source.ContentType(),
		noRemoveClose{tmp},
	)
	return file, cleanup, nil
}

// noRemoveClose leaves lifetime control with spooledPackage.
type noRemoveClose struct{ *os.File }

func (noRemoveClose) Close() error { return nil }
