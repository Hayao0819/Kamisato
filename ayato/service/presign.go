package service

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

// archFromFilename derives the storage arch from a pacman package filename
// (<pkgname>-<pkgver>-<pkgrel>-<arch>.pkg.tar.<ext>, optionally .sig): it is the
// last '-'-separated field before ".pkg.tar". Presign runs before any bytes exist
// to read a .PKGINFO from, so the arch has to come from the name the client sends.
func archFromFilename(filename string) (string, error) {
	name := strings.TrimSuffix(path.Base(filename), ".sig")
	i := strings.Index(name, ".pkg.tar")
	if i <= 0 {
		return "", fmt.Errorf("%w: %q is not a package filename", domain.ErrInvalidUpload, filename)
	}
	stem := name[:i]
	j := strings.LastIndex(stem, "-")
	if j < 0 || j == len(stem)-1 {
		return "", fmt.Errorf("%w: cannot derive arch from %q", domain.ErrInvalidUpload, filename)
	}
	return stem[j+1:], nil
}

// PresignUploads issues a presigned PUT URL for each filename so the client can
// upload straight to R2, bypassing the request-body limit in front of the server.
// It presigns to the final key; FinalizeUploads validates and registers the object
// afterwards. A backend that cannot presign returns blob.ErrPresignUnsupported
// unwrapped, so the handler can answer 501 and the client falls back to multipart.
func (s *Service) PresignUploads(repo string, filenames []string) (map[string]string, error) {
	repo = s.publishTarget(repo)
	urls := make(map[string]string, len(filenames))
	for _, filename := range filenames {
		arch, err := archFromFilename(filename)
		if err != nil {
			return nil, err
		}
		// A concrete-arch package presigned to an arch the repo does not serve would
		// let the client PUT an object finalize then rejects; gate it up front. A
		// .sig rides its package's arch, so it needs no separate gate.
		if !strings.HasSuffix(filename, ".sig") && arch != "any" && !s.archAccepted(repo, arch) {
			return nil, fmt.Errorf("%w: arch %q is not served by repo %q", domain.ErrInvalidUpload, arch, repo)
		}
		url, err := s.pkgBinaryRepo.StoreFileWithSignedPutURL(repo, arch, filename)
		if err != nil {
			if errors.Is(err, blob.ErrPresignUnsupported) {
				return nil, err
			}
			return nil, errors.WrapErr(err, "failed to presign upload")
		}
		urls[filename] = url
	}
	return urls, nil
}

// FinalizeUploads registers packages the client already PUT to R2. It sources each
// package (and its .sig) from R2 instead of a multipart form, spools them to
// re-seekable temp files, and reuses the exact validate+register path without
// re-storing the bytes. On any registration failure it deletes the just-PUT R2
// objects so a rejected upload leaves nothing behind.
func (s *Service) FinalizeUploads(repo string, pkgFilenames []string) error {
	repo = s.publishTarget(repo)

	var files []*domain.UploadFiles
	var cleanups []func()
	defer func() {
		for _, c := range cleanups {
			c()
		}
	}()

	var stored []finalizedObj

	for _, name := range pkgFilenames {
		arch, err := archFromFilename(name)
		if err != nil {
			return err
		}
		stored = append(stored, finalizedObj{arch, name})

		pkgStream, pkgCleanup, err := s.spoolFetched(repo, arch, name)
		if err != nil {
			s.cleanupFinalized(repo, stored)
			return errors.WrapErr(err, "fetch finalized package from store")
		}
		cleanups = append(cleanups, pkgCleanup)

		uf := &domain.UploadFiles{PkgFile: pkgStream}
		sigStream, sigCleanup, serr := s.spoolFetched(repo, arch, name+".sig")
		if serr == nil {
			cleanups = append(cleanups, sigCleanup)
			uf.SigFile = sigStream
		} else if !errors.Is(serr, blob.ErrNotFound) {
			s.cleanupFinalized(repo, stored)
			return errors.WrapErr(serr, "fetch finalized signature from store")
		}
		files = append(files, uf)
	}

	if err := s.uploadFiles(repo, files, false); err != nil {
		s.cleanupFinalized(repo, stored)
		return err
	}
	return nil
}

// spoolFetched fetches an object from the store and copies it into a re-seekable
// temp file: R2's FetchFile stream is not seekable, but prepareUpload/RepoAddBatch
// rewind it. The returned SeekFile keeps the object's name so the register path
// keys it correctly.
func (s *Service) spoolFetched(repo, arch, name string) (stream.SeekFile, func(), error) {
	f, err := s.pkgBinaryRepo.FetchFile(repo, arch, name)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	tmp, err := os.CreateTemp("", "ayato-finalize-")
	if err != nil {
		return nil, nil, err
	}
	remove := func() { _ = tmp.Close(); _ = os.Remove(tmp.Name()) }
	if _, err := io.Copy(tmp, f); err != nil {
		remove()
		return nil, nil, errors.WrapErr(err, "spool finalized file")
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		remove()
		return nil, nil, err
	}
	return stream.NewFileStream(path.Base(name), f.ContentType(), noRemoveClose{tmp}), remove, nil
}

// cleanupFinalized deletes the R2 objects (package and its .sig) for every package
// finalize touched, undoing the client's direct PUTs on a failed finalize. Deleting
// a missing key is a no-op.
// finalizedObj is one (arch, package-name) whose R2 objects finalize created.
type finalizedObj struct{ arch, name string }

func (s *Service) cleanupFinalized(repo string, stored []finalizedObj) {
	for _, f := range stored {
		if err := s.pkgBinaryRepo.DeleteFile(repo, f.arch, f.name); err != nil {
			slog.Warn("failed to clean up finalized package after error", "repo", repo, "arch", f.arch, "name", f.name, "err", err)
		}
		if err := s.pkgBinaryRepo.DeleteFile(repo, f.arch, f.name+".sig"); err != nil {
			slog.Warn("failed to clean up finalized signature after error", "repo", repo, "arch", f.arch, "name", f.name+".sig", "err", err)
		}
	}
}
