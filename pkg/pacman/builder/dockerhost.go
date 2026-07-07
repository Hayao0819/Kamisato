package builder

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/client"
)

// newDockerClient resolves the daemon endpoint (host → DOCKER_HOST → active context → socket), matching docker CLI priority.
// client.FromEnv alone ignores contexts, causing wrong-daemon issues on Docker Desktop/rootless/remote setups.
func newDockerClient(host string) (*client.Client, error) {
	opts := []client.Opt{client.WithAPIVersionNegotiation(), client.FromEnv}
	if host == "" && os.Getenv("DOCKER_HOST") == "" {
		host = dockerHostFromContext()
	}
	if host != "" {
		opts = append(opts, client.WithHost(host))
	}
	return client.NewClientWithOpts(opts...)
}

// dockerHostFromContext reads ~/.docker to get the active context's endpoint; returns "" for the default context or on error (callers fall back to socket).
func dockerHostFromContext() string {
	dir := os.Getenv("DOCKER_CONFIG")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".docker")
	}

	name := os.Getenv("DOCKER_CONTEXT")
	if name == "" {
		data, err := os.ReadFile(filepath.Join(dir, "config.json")) //nolint:gosec // dir is the operator's own DOCKER_CONFIG/~/.docker, mirroring the docker CLI
		if err != nil {
			return ""
		}
		var cfg struct {
			CurrentContext string `json:"currentContext"`
		}
		if json.Unmarshal(data, &cfg) != nil {
			return ""
		}
		name = cfg.CurrentContext
	}
	if name == "" || name == "default" {
		return ""
	}

	// The context store keys each context by the hex sha256 of its name.
	id := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))
	data, err := os.ReadFile(filepath.Join(dir, "contexts", "meta", id, "meta.json")) //nolint:gosec // dir is the operator's own DOCKER_CONFIG/~/.docker, mirroring the docker CLI
	if err != nil {
		return ""
	}
	var meta struct {
		Endpoints map[string]struct {
			Host string `json:"Host"`
		} `json:"Endpoints"`
	}
	if json.Unmarshal(data, &meta) != nil {
		return ""
	}
	return meta.Endpoints["docker"].Host
}
