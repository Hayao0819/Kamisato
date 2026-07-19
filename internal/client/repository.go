package client

import (
	"bytes"
	"context"
	"net/http"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

const maxRepositoryDatabaseBytes = 128 << 20

// Repository is the public, read-only Ayato repository client.
type Repository struct{ transport *transport }

func NewRepository(base string, opts ...Option) (*Repository, error) {
	transport, err := newTransport(base, noCredential{}, opts...)
	if err != nil {
		return nil, err
	}
	return &Repository{transport: transport}, nil
}

// Database fetches and parses /repo/<repo>/<arch>/<repo>.db. A missing
// repository is the normal bootstrap state and is represented by an empty
// RemoteRepo; all other transport and protocol failures remain errors.
func (c *Repository) Database(ctx context.Context, repoName, arch string) (*repo.RemoteRepo, error) {
	if repoName == "" {
		return nil, errors.NewErr("repository name is required")
	}
	if arch == "" {
		return nil, errors.NewErr("repository architecture is required")
	}

	body, err := c.transport.readBytes(
		ctx,
		c.transport.endpoint("repo", repoName, arch, repo.Artifacts(repoName).DatabaseAlias()),
		maxRepositoryDatabaseBytes,
		"fetch Ayato repository database",
		"application/octet-stream",
	)
	if err != nil {
		var responseErr *ResponseError
		if errors.As(err, &responseErr) && responseErr.StatusCode == http.StatusNotFound {
			return &repo.RemoteRepo{Name: repoName}, nil
		}
		return nil, err
	}

	database, err := repo.RemoteRepoFromDBContext(ctx, repoName, bytes.NewReader(body))
	if err != nil {
		return nil, errors.WrapErr(err, "parse Ayato repository database")
	}
	return database, nil
}
