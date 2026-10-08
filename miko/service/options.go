package service

import (
	"context"
	"net/http"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
	"github.com/Hayao0819/Kamisato/internal/pacman/depend"
	"github.com/Hayao0819/Kamisato/internal/pacman/repo"
	"github.com/Hayao0819/Kamisato/internal/pacman/sign"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

// RepositoryDBReader is the repository view Miko needs from Ayato. Keeping the
// port here prevents build orchestration from constructing URLs or handling
// HTTP response policy itself.
type RepositoryDBReader interface {
	Database(ctx context.Context, repoName, arch string) (*repo.RemoteRepo, error)
}

// AURClient is the upstream metadata and clone-location boundary used when
// resolving and authorizing recursive AUR dependencies.
type AURClient interface {
	Info(context.Context, []string) ([]aurweb.Pkg, error)
	Search(context.Context, aurweb.By, string) ([]aurweb.Pkg, error)
	GitBase() string
}

// ServiceOption configures a Miko service collaborator.
type ServiceOption func(*Service)

// WithBuildBackend supplies the per-job backend factory. Configuration remains
// resolved from trusted host settings before this boundary is called.
func WithBuildBackend(create func(builder.ResolvedConfig) (builder.Backend, error)) ServiceOption {
	return func(options *Service) { options.newBackend = create }
}

// WithAURDependencies supplies the metadata and host-repository collaborators
// used by recursive dependency builds.
func WithAURDependencies(upstream AURClient, repositories depend.RepoChecker) ServiceOption {
	return func(options *Service) {
		options.aur = upstream
		options.repoChecker = repositories
	}
}

// WithSonameStore enables durable ABI history without opening files in New.
func WithSonameStore(store SonameStore) ServiceOption {
	return func(options *Service) { options.sonames = store }
}

// WithSigner enables package signing with signer.
func WithSigner(signer sign.Signer) ServiceOption {
	return func(options *Service) {
		options.signer = signer
	}
}

// WithPersister enables durable job persistence with persister.
func WithPersister(persister Persister) ServiceOption {
	return func(options *Service) {
		options.persist = persister
	}
}

// WithUploader enables publication of successful builds with uploader.
func WithUploader(uploader Uploader) ServiceOption {
	return func(options *Service) {
		options.uploader = uploader
	}
}

// WithOutboundHTTPClient supplies the client used for upstream version checks.
func WithOutboundHTTPClient(client *http.Client) ServiceOption {
	return func(options *Service) {
		if client != nil {
			options.httpClient = client
		}
	}
}

// WithRepositoryDBReader supplies Ayato's public repository database reader.
func WithRepositoryDBReader(reader RepositoryDBReader) ServiceOption {
	return func(options *Service) {
		options.repositories = reader
	}
}

func (s *Service) repositoryDB(ctx context.Context, repoName, arch string) (*repo.RemoteRepo, error) {
	if s.repositories == nil {
		return nil, errors.NewErr("Ayato repository reader is not configured")
	}
	return s.repositories.Database(ctx, repoName, arch)
}
