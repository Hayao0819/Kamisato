package repository

import (
	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/platform"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	pacmanpkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// FetchFile resolves public database aliases and the shared arch=any object
// directory while keeping the blob.Store free of pacman naming knowledge.
func (r *binaryRepository) FetchFile(
	repo, arch, file string,
) (platform.File, error) {
	result, err := r.resolveFile(repo, arch, file, func(repo, arch, file string) (fileResult, error) {
		object, err := r.Store.FetchFile(repo, arch, file)
		return fileResult{file: object}, err
	})
	return result.file, err
}

// FetchFileWithMeta applies the same resolution as FetchFile and carries the
// selected physical object's validators to the HTTP layer.
func (r *binaryRepository) FetchFileWithMeta(
	repo, arch, file string,
) (platform.File, blob.FileMeta, error) {
	fetcher, ok := r.Store.(blob.MetaFetcher)
	if !ok {
		object, err := r.FetchFile(repo, arch, file)
		return object, blob.FileMeta{}, err
	}
	result, err := r.resolveFile(repo, arch, file, func(repo, arch, file string) (fileResult, error) {
		object, meta, err := fetcher.FetchFileWithMeta(repo, arch, file)
		return fileResult{file: object, meta: meta}, err
	})
	return result.file, result.meta, err
}

type fileResult struct {
	file platform.File
	meta blob.FileMeta
}

func (r *binaryRepository) resolveFile(
	repo, arch, file string,
	fetch func(repo, arch, file string) (fileResult, error),
) (fileResult, error) {
	object, err := fetch(repo, arch, file)
	if err == nil {
		return object, nil
	}
	for _, target := range r.dbAliasTargets(repo, file) {
		alias, aliasErr := fetch(repo, arch, target)
		if aliasErr == nil {
			alias.file = aliasFile{File: alias.file, name: file}
			return alias, nil
		}
		if !errors.Is(aliasErr, blob.ErrNotFound) {
			return fileResult{}, aliasErr
		}
	}
	if arch == "any" || !pacmanpkg.IsAny(file) {
		return object, err
	}
	shared, sharedErr := fetch(repo, "any", file)
	if sharedErr == nil {
		return shared, nil
	}
	return object, err
}

func dbAliasArchive(
	artifacts pacmanrepo.DatabaseArtifacts,
	file string,
) string {
	target, ok := artifacts.ArchiveForAlias(file)
	if !ok {
		return ""
	}
	return target
}

func (r *binaryRepository) isUpstreamRepo(repo string) bool {
	return r.upstream[repo]
}

// dbAliasTargets prefers a synthesized merged archive for upstream repositories
// and falls back to the local overlay before the first synchronization.
func (r *binaryRepository) dbAliasTargets(repo, file string) []string {
	overlay := dbAliasArchive(pacmanrepo.Artifacts(repo), file)
	if overlay == "" {
		return nil
	}
	if r.isUpstreamRepo(repo) {
		return []string{
			dbAliasArchive(mergedArtifacts(repo), file),
			overlay,
		}
	}
	return []string{overlay}
}

type aliasFile struct {
	platform.File
	name string
}

func (a aliasFile) FileName() string { return a.name }

// StoreFileWithSignedURL resolves logical aliases to physical stored objects. A
// synthesized upstream DB must be streamed so FetchFile's fallback stays active.
func (r *binaryRepository) StoreFileWithSignedURL(
	repo, arch, name string,
) (string, error) {
	if overlay := dbAliasArchive(pacmanrepo.Artifacts(repo), name); overlay != "" {
		if r.isUpstreamRepo(repo) {
			return "", nil
		}
		name = overlay
	} else if arch != "any" && pacmanpkg.IsAny(name) {
		arch = "any"
	}
	return r.Store.StoreFileWithSignedURL(repo, arch, name)
}
