package builder

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/client"
)

// newDockerClient resolves the daemon endpoint with the same priority the
// docker CLI uses: explicit host, then DOCKER_HOST, then the active docker
// context, then the default socket. client.FromEnv alone ignores contexts, so a
// host using Docker Desktop / rootless / a remote context would otherwise hit
// the wrong daemon.
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

// dockerHostFromContext returns the docker endpoint of the active context by
// reading ~/.docker (config.json + the context metadata store). It returns ""
// for the default context or on any error, so callers fall back to the socket.
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
		data, err := os.ReadFile(filepath.Join(dir, "config.json"))
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
	data, err := os.ReadFile(filepath.Join(dir, "contexts", "meta", id, "meta.json"))
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
