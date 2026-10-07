package buildset

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/pacman/depend"
	"github.com/Hayao0819/Kamisato/internal/pacman/host"
	pacmanpkg "github.com/Hayao0819/Kamisato/internal/pacman/pkg"
	"github.com/Hayao0819/Kamisato/internal/pacman/repo"
)

type repositorySnapshot struct {
	name     string
	packages []*pacmanpkg.BinaryPackage
}

var repositoryHTTPClient = &http.Client{Timeout: 45 * time.Second}

func loadRepositories(ctx context.Context, config *host.BuildConfig) ([]repositorySnapshot, error) {
	var snapshots []repositorySnapshot
	for _, configured := range config.Repositories {
		if !repositoryInstallsPackages(configured.Usage) {
			continue
		}
		if len(configured.Servers) == 0 {
			return nil, fmt.Errorf("repository %q has no Server entries", configured.Name)
		}
		var failures []error
		loaded := false
		for _, server := range configured.Servers {
			packages, err := fetchRepository(ctx, configured.Name, server)
			if err != nil {
				failures = append(failures, err)
				continue
			}
			snapshots = append(snapshots, repositorySnapshot{name: configured.Name, packages: packages})
			loaded = true
			break
		}
		if !loaded {
			return nil, fmt.Errorf("load repository %q: %w", configured.Name, errors.Join(failures...))
		}
	}
	return snapshots, nil
}

func configuredLocalRepositoryDirs(config *host.BuildConfig) ([]string, error) {
	var directories []string
	for _, repository := range config.Repositories {
		for _, server := range repository.Servers {
			path, local, err := localRepositoryPath(server)
			if err != nil {
				return nil, fmt.Errorf("repository %q server %q: %w", repository.Name, server, err)
			}
			if local {
				info, err := os.Stat(path)
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				if err != nil {
					return nil, fmt.Errorf("inspect repository %q server %q: %w", repository.Name, server, err)
				}
				if !info.IsDir() {
					return nil, fmt.Errorf("repository %q server %q is not a directory", repository.Name, server)
				}
				directories = append(directories, path)
			}
		}
	}
	slices.Sort(directories)
	return slices.Compact(directories), nil
}

func repositoryInstallsPackages(usage []string) bool {
	return len(usage) == 0 || slices.Contains(usage, "All") || slices.Contains(usage, "Install")
}

func fetchRepository(ctx context.Context, name, server string) ([]*pacmanpkg.BinaryPackage, error) {
	location, err := url.JoinPath(server, name+".db")
	if err != nil {
		return nil, fmt.Errorf("repository %q server %q: %w", name, server, err)
	}
	parsed, err := url.Parse(location)
	if err != nil {
		return nil, fmt.Errorf("repository %q database URL %q: %w", name, location, err)
	}
	var reader io.ReadCloser
	switch parsed.Scheme {
	case "file":
		path, _, err := localRepositoryPath(location)
		if err != nil {
			return nil, err
		}
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open repository %q database %q: %w", name, path, err)
		}
		reader = file
	case "http", "https":
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
		if err != nil {
			return nil, err
		}
		response, err := repositoryHTTPClient.Do(request) //nolint:gosec // servers come from the operator's explicit pacman.conf
		if err != nil {
			return nil, fmt.Errorf("download repository %q database from %q: %w", name, location, err)
		}
		if response.StatusCode != http.StatusOK {
			_ = response.Body.Close()
			return nil, fmt.Errorf("download repository %q database from %q: HTTP %s", name, location, response.Status)
		}
		reader = response.Body
	default:
		return nil, fmt.Errorf("repository %q uses unsupported Server scheme %q", name, parsed.Scheme)
	}
	defer reader.Close()
	remote, err := repo.RemoteRepoFromDBContext(ctx, name, reader)
	if err != nil {
		return nil, fmt.Errorf("parse repository %q database from %q: %w", name, location, err)
	}
	return remote.Pkgs, nil
}

func localRepositoryPath(server string) (string, bool, error) {
	parsed, err := url.Parse(server)
	if err != nil {
		return "", false, err
	}
	if parsed.Scheme != "file" {
		return "", false, nil
	}
	if parsed.Host != "" && parsed.Host != "localhost" {
		return "", false, fmt.Errorf("unsupported file URL host %q", parsed.Host)
	}
	decoded, err := url.PathUnescape(parsed.Path)
	if err != nil {
		return "", false, err
	}
	absolute, err := filepath.Abs(filepath.FromSlash(decoded))
	if err != nil {
		return "", false, err
	}
	return absolute, true, nil
}

func repositoryCandidates(snapshot repositorySnapshot, spec, arch string) []string {
	constraint := depend.Parse(spec)
	var candidates []string
	for _, pkg := range snapshot.packages {
		if pkg.Arch() != "" && pkg.Arch() != "any" && pkg.Arch() != arch {
			continue
		}
		if pkg.Name() == constraint.Name {
			matches, _ := constraint.Satisfies(pkg.Version())
			if matches {
				candidates = append(candidates, pkg.Name())
				continue
			}
		}
		for _, provided := range pkg.PKGINFO().Provides {
			if providedSatisfies(constraint, provided) {
				candidates = append(candidates, pkg.Name())
				break
			}
		}
	}
	slices.Sort(candidates)
	return slices.Compact(candidates)
}

func providedSatisfies(constraint depend.Constraint, provided string) bool {
	parsed := depend.Parse(strings.TrimSpace(provided))
	if parsed.Name != constraint.Name {
		return false
	}
	if constraint.Op == depend.OpAny {
		return true
	}
	if parsed.Op == depend.OpAny || parsed.Ver == "" {
		return false
	}
	matches, err := constraint.Satisfies(parsed.Ver)
	return err == nil && matches
}
