package repository

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

func withRepoDBTempDir(pattern string, run func(dir string) error) error {
	dir, err := os.MkdirTemp("", pattern)
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	return run(dir)
}

// writeSeekFileTo materializes a stream under its base name. A nil stream is a
// supported no-op because signatures are optional.
func writeSeekFileTo(dir string, file stream.SeekFile) (string, error) {
	if file == nil {
		return "", nil
	}
	dst := path.Join(dir, path.Base(file.FileName()))
	if err := writeSeekFileToPath(dst, file); err != nil {
		return "", err
	}
	return dst, nil
}

func writeSeekFileToPath(dst string, file stream.SeekFile) error {
	if file == nil {
		return nil
	}
	// Reuse an already materialized file when both paths are on one filesystem.
	if diskFile, ok := file.(stream.OnDiskFile); ok {
		if err := os.Link(diskFile.OnDiskPath(), dst); err == nil {
			return nil
		}
	}
	if err := stream.Rewind(file); err != nil {
		return errors.WrapErr(err, "failed to seek stream")
	}
	return writeReaderToPath(dst, file)
}

func writeReaderToPath(dst string, src io.Reader) error {
	out, err := os.Create(dst)
	if err != nil {
		return errors.WrapErr(err, "failed to create temp file")
	}
	_, copyErr := io.Copy(out, src)
	closeErr := out.Close()
	if copyErr != nil {
		return errors.WrapErr(copyErr, "failed to copy stream to temp file")
	}
	if closeErr != nil {
		return errors.WrapErr(closeErr, "failed to close temp file")
	}
	return nil
}

func writeReaderAndClose(dst string, src io.ReadCloser) error {
	writeErr := writeReaderToPath(dst, src)
	closeErr := src.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return errors.WrapErr(closeErr, "failed to close source file")
	}
	return nil
}

func copyLocalFile(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	return writeReaderAndClose(dst, in)
}

func (r *binaryRepository) materializePackages(
	repoName, arch, dir string,
	filenames []string,
) ([]string, error) {
	pkgDir := path.Join(dir, "packages")
	if err := os.MkdirAll(pkgDir, 0o700); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(filenames))
	seen := make(map[string]struct{}, len(filenames))
	for _, filename := range filenames {
		if path.Base(filename) != filename {
			return nil, fmt.Errorf("invalid canonical package filename %q", filename)
		}
		if _, exists := seen[filename]; exists {
			continue
		}
		seen[filename] = struct{}{}
		pkgFile, err := r.FetchFile(repoName, arch, filename)
		if err != nil {
			return nil, errors.WrapErr(err, "fetch canonical package object "+filename)
		}
		dst := path.Join(pkgDir, filename)
		if err := writeReaderAndClose(dst, pkgFile); err != nil {
			return nil, errors.WrapErr(err, "materialize canonical package object "+filename)
		}
		paths = append(paths, dst)
	}
	return paths, nil
}

// clearDBArtifacts drops data seeded by the previous retry while retaining the
// caller's package files.
func clearDBArtifacts(dir, repoName string, useSignedDB bool) error {
	artifacts := pacmanrepo.Artifacts(repoName)
	names := append([]string{}, artifacts.Archives()...)
	names = append(names, artifacts.Aliases()...)
	if useSignedDB {
		names = append(names, artifacts.ArchiveSignatures()...)
		names = append(names, artifacts.AliasSignatures()...)
	}
	for _, name := range names {
		if err := os.Remove(path.Join(dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return errors.WrapErr(err, "failed to clear stale db artifact "+name)
		}
	}
	return nil
}

// fetchDBArtifacts seeds a mutation workspace and records each object's ETag.
// Missing files are valid for a fresh database; other backend failures abort the
// attempt so an outage can never be mistaken for an empty repository.
func (r *binaryRepository) fetchDBArtifacts(
	repo, arch, dir string,
	useSignedDB bool,
) (map[string]string, error) {
	names := dbArtifactBases(repo)
	if useSignedDB {
		for _, name := range dbArtifactBases(repo) {
			names = append(names, name+".sig")
		}
	}
	etags := make(map[string]string, len(names))
	for _, name := range names {
		file, etag, err := r.Store.FetchFileWithETag(repo, arch, name)
		if errors.Is(err, blob.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, errors.WrapErr(err, "failed to fetch db artifact "+name)
		}
		if err := writeReaderAndClose(path.Join(dir, name), file); err != nil {
			return nil, errors.WrapErr(err, "failed to materialize db artifact "+name)
		}
		etags[name] = etag
	}
	return etags, nil
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}
