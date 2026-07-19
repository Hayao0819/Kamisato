package repository

import (
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/samber/lo"

	"github.com/Hayao0819/Kamisato/ayato/platform"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

//go:generate mockgen -source=repository.go -destination=../test/mocks/repository.go -package=mocks -aux_files=github.com/Hayao0819/Kamisato/ayato/repository/blob=blob/blob.go

// BinaryRepository is the service layer's port for package objects, pacman
// databases, and their derived/upstream representations.
type BinaryRepository interface {
	blob.Store
	StoreFileImmutable(
		name, arch string,
		file platform.SeekFile,
	) (created bool, err error)
	DeleteOrphanIfUnchanged(
		name, arch string,
		expected blob.FileInfo,
		cutoff time.Time,
	) (deleted bool, err error)
	RepoAdd(
		name, arch string,
		pkg, sig platform.SeekFile,
		useSignedDB bool,
		gnupgDir *string,
	) error
	RepoAddBatch(
		name, arch string,
		items []RepoAddItem,
		useSignedDB bool,
		gnupgDir *string,
	) error
	RepoRemove(
		name, arch, pkg string,
		useSignedDB bool,
		gnupgDir *string,
	) error
	RepoRemoveIfMatch(
		name, arch, pkg, expectedVersion, expectedFile string,
		useSignedDB bool,
		gnupgDir *string,
	) error
	ReconcileDB(name, arch string, useSignedDB bool, gnupgDir *string) error
	InitArch(name, arch string, useSignedDB bool, gnupgDir *string) error
	BackfillSignatures(name, arch string) error
	RebuildMerged(name, arch string, useSignedDB bool) error
	ApplyUpstreamSnapshot(
		name, arch string,
		dbGz, filesGz []byte,
		etag, lastModified string,
		useSignedDB bool,
	) (pacmanrepo.DBDiff, error)
	UpstreamValidators(name, arch string) (etag, lastModified string, err error)
	FetchDB(repoName, archName string) (platform.File, error)
	FetchFileWithMeta(
		repo, arch, file string,
	) (platform.File, blob.FileMeta, error)
	PkgNames(repoName, archName string) ([]string, error)
	RemoteRepo(name, arch string) (*pacmanrepo.RemoteRepo, error)
	PkgFiles(repoName, archName, pkgName string) ([]string, error)
	VerifyPkgRepo(name string) error
}

// binaryRepository composes the raw blob adapter with pacman-domain operations.
type binaryRepository struct {
	blob.Store
	dbMu keyedMutex
	// nil selects pacmanrepo.NativeTool.
	tool repoDBTool
	// dbSigner signs synthesized upstream/overlay databases.
	dbSigner *openpgp.Entity
	// upstream marks repositories whose public DB is a merged view.
	upstream map[string]bool
}

type BinaryRepoOption func(*binaryRepository)

func WithSigningTool(entity *openpgp.Entity) BinaryRepoOption {
	return func(repository *binaryRepository) {
		if entity != nil {
			repository.tool = pacmanrepo.NewSigningNativeTool(entity)
			repository.dbSigner = entity
		}
	}
}

func WithUpstreamRepos(names []string) BinaryRepoOption {
	return func(repository *binaryRepository) {
		if len(names) == 0 {
			return
		}
		repository.upstream = make(map[string]bool, len(names))
		for _, name := range names {
			repository.upstream[name] = true
		}
	}
}

func NewBinaryRepository(
	store blob.Store,
	options ...BinaryRepoOption,
) BinaryRepository {
	repository := &binaryRepository{Store: store}
	for _, option := range options {
		if option != nil {
			option(repository)
		}
	}
	return repository
}

// Arches omits the internal any/ object directory. arch=any packages are served
// through each concrete architecture's database.
func (r *binaryRepository) Arches(name string) ([]string, error) {
	arches, err := r.Store.Arches(name)
	if err != nil {
		return nil, err
	}
	return lo.Filter(arches, func(arch string, _ int) bool {
		return arch != "any"
	}), nil
}
