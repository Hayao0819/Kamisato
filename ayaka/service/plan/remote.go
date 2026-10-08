package plan

import (
	"context"
	"net/http"
	"strings"

	"github.com/Hayao0819/Kamisato/ayaka/source"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/pacman/repo"
)

// ResolveDiffServer picks the remote repo db dir: the explicit --diff-url, else
// the deprecated --server, else the arch-less repo.json url with the arch
// appended. Empty when none is configured.
func ResolveDiffServer(diffURL, server, configURL, arch string) string {
	if diffURL != "" {
		return diffURL
	}
	if server != "" {
		return server
	}
	if configURL != "" {
		return strings.TrimRight(configURL, "/") + "/" + arch
	}
	return ""
}

// RemoteRepo fetches the published repo db per ResolveDiffServer; a repo/arch
// with no db yet resolves to an empty repo, so first runs plan/build everything.
func RemoteRepo(ctx context.Context, client *http.Client, diffURL, server string, src *source.SourceRepo, arch string) (*repo.RemoteRepo, error) {
	dburl := ResolveDiffServer(diffURL, server, src.Config.URL, arch)
	if dburl == "" {
		return nil, errors.NewErr("source repo " + src.Config.Name + " has no url in repo.json; pass --diff-url")
	}
	rr, err := repo.FetchOrEmpty(ctx, client, dburl, src.Config.Name)
	if err != nil {
		return nil, errors.WrapErr(err, "failed to read remote repo db")
	}
	return rr, nil
}
