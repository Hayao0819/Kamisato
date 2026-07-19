package repository

import (
	"fmt"
	"path"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/samber/lo"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/platform"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

func (r *binaryRepository) FetchDB(
	repoName, archName string,
) (platform.File, error) {
	return r.FetchFile(
		repoName,
		archName,
		pacmanrepo.Artifacts(repoName).DatabaseAlias(),
	)
}

func (r *binaryRepository) RemoteRepo(
	name, arch string,
) (*pacmanrepo.RemoteRepo, error) {
	return r.remoteRepoFromArtifact(
		name,
		arch,
		pacmanrepo.Artifacts(name).DatabaseAlias(),
	)
}

func (r *binaryRepository) remoteRepoFromArtifact(
	name, arch, artifact string,
) (*pacmanrepo.RemoteRepo, error) {
	database, err := r.FetchFile(name, arch, artifact)
	if err != nil {
		return nil, err
	}
	defer database.Close()
	remote, err := pacmanrepo.RemoteRepoFromDB(name, database)
	if err != nil {
		return nil, err
	}
	if remote == nil {
		return nil, fmt.Errorf("failed to get repository")
	}
	return remote, nil
}

func (r *binaryRepository) PkgNames(
	repoName, archName string,
) ([]string, error) {
	remote, err := r.remoteRepoFromArtifact(
		repoName,
		archName,
		pacmanrepo.Artifacts(repoName).DatabaseArchive(),
	)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(remote.Pkgs))
	for _, pkg := range remote.Pkgs {
		names = append(names, pkg.Base())
	}
	return names, nil
}

func (r *binaryRepository) PkgFiles(
	repoName, archName, pkgName string,
) ([]string, error) {
	database, err := r.FetchFile(
		repoName,
		archName,
		pacmanrepo.Artifacts(repoName).FilesArchive(),
	)
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, errors.WrapErr(err, "failed to fetch files db")
	}
	defer database.Close()

	filesByPackage, err := pacmanrepo.FilesFromDB(database)
	if err != nil {
		return nil, errors.WrapErr(err, "failed to parse files db")
	}
	files, ok := filesByPackage[pkgName]
	if !ok {
		return nil, fmt.Errorf(
			"%w: package %q has no files entry in %s/%s",
			domain.ErrNotFound,
			pkgName,
			repoName,
			archName,
		)
	}
	return files, nil
}

func (r *binaryRepository) VerifyPkgRepo(name string) error {
	arches, err := r.Arches(name)
	if err != nil {
		return errors.WrapErr(err, "failed to get arches")
	}
	for _, arch := range arches {
		files, err := r.Files(name, arch)
		if err != nil {
			return errors.WrapErr(
				err,
				fmt.Sprintf("failed to get files for arch %s", arch),
			)
		}
		for _, artifact := range pacmanrepo.Artifacts(name).Archives() {
			if !lo.Contains(files, artifact) {
				return fmt.Errorf("%s not found in %s", artifact, path.Join(name, arch))
			}
		}
	}
	return nil
}
