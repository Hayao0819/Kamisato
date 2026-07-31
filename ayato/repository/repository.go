package repository

import (
	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/samber/lo"

	"github.com/Hayao0819/Kamisato/ayato/blob"
	"github.com/Hayao0819/Kamisato/ayato/service"
	pacmanrepo "github.com/Hayao0819/Kamisato/internal/pacman/repo"
)

//go:generate mockgen -source=repository.go -destination=../test/mocks/repository.go -package=mocks -aux_files=github.com/Hayao0819/Kamisato/ayato/blob=blob/blob.go

// BinaryRepository is the service layer's port for package objects, pacman
// databases, and their derived/upstream representations.
type BinaryRepository = service.BinaryRepository

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
	// staged is nil unless the raw store (pre-serialization wrapper) supports
	// blob.StagedUploader; staged uploads bypass the serializing wrapper because
	// their key layout is not (repo, arch) and does not need that mutex.
	staged blob.StagedUploader
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

// WithStagedUploads probes rawStore for blob.StagedUploader. Pass the store
// from before serialization wrapping: the wrapper only forwards blob.Store, so
// probing it here would always report the capability absent.
func WithStagedUploads(rawStore blob.Store) BinaryRepoOption {
	return func(repository *binaryRepository) {
		repository.staged, _ = rawStore.(blob.StagedUploader)
	}
}

func (r *binaryRepository) StagedUploads() (blob.StagedUploader, bool) {
	return r.staged, r.staged != nil
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
