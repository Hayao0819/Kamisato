package service

import (
	"io"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
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
	pkg      stream.SeekFile
	sig      stream.SeekFile
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

func closeSpooledPackages(artifacts []*spooledPackage) {
	for _, artifact := range artifacts {
		artifact.close()
	}
}

func (s *Service) spoolRepositoryFile(
	repo, arch, filename string,
) (stream.SeekFile, func(), error) {
	source, err := s.pkgBinaryRepo.FetchFile(repo, arch, filename)
	if err != nil {
		return nil, nil, err
	}
	defer source.Close()

	tmp, err := os.CreateTemp("", "ayato-publication-")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}
	if _, err := io.Copy(tmp, source); err != nil {
		cleanup()
		return nil, nil, errors.WrapErr(err, "spool repository file")
	}
	if err := stream.Rewind(tmp); err != nil {
		cleanup()
		return nil, nil, err
	}
	file := stream.NewFileStream(
		path.Base(filename),
		source.ContentType(),
		noRemoveClose{tmp},
	)
	return file, cleanup, nil
}

// noRemoveClose leaves lifetime control with spooledPackage.
type noRemoveClose struct{ *os.File }

func (noRemoveClose) Close() error { return nil }
