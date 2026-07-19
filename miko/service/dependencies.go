package service

import (
	"context"
	"net/http"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// RepositoryDBReader is the repository view Miko needs from Ayato. Keeping the
// port here prevents build orchestration from constructing URLs or handling
// HTTP response policy itself.
type RepositoryDBReader interface {
	Database(ctx context.Context, repoName, arch string) (*repo.RemoteRepo, error)
}

type serviceOptions struct {
	httpClient   *http.Client
	repositories RepositoryDBReader
}

// ServiceOption configures an optional Miko service dependency.
type ServiceOption func(*serviceOptions)

// WithOutboundHTTPClient supplies the client used for upstream version checks.
func WithOutboundHTTPClient(client *http.Client) ServiceOption {
	return func(options *serviceOptions) {
		if client != nil {
			options.httpClient = client
		}
	}
}

// WithRepositoryDBReader supplies Ayato's public repository database reader.
func WithRepositoryDBReader(reader RepositoryDBReader) ServiceOption {
	return func(options *serviceOptions) {
		options.repositories = reader
	}
}

func (s *Service) repositoryDB(ctx context.Context, repoName, arch string) (*repo.RemoteRepo, error) {
	if s.repositories == nil {
		return nil, errors.NewErr("Ayato repository reader is not configured")
	}
	return s.repositories.Database(ctx, repoName, arch)
}
