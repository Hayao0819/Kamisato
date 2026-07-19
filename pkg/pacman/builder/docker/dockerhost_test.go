package docker

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestDockerHostFromContext(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	t.Setenv("DOCKER_CONTEXT", "")
	t.Setenv("DOCKER_HOST", "")

	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"currentContext":"testctx"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	id := fmt.Sprintf("%x", sha256.Sum256([]byte("testctx")))
	metaDir := filepath.Join(dir, "contexts", "meta", id)
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := `{"Name":"testctx","Endpoints":{"docker":{"Host":"unix:///x/y.sock"}}}`
	if err := os.WriteFile(filepath.Join(metaDir, "meta.json"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := dockerHostFromContext(); got != "unix:///x/y.sock" {
		t.Errorf("dockerHostFromContext() = %q, want unix:///x/y.sock", got)
	}
}

func TestDockerHostFromContextDefault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	t.Setenv("DOCKER_CONTEXT", "")

	if got := dockerHostFromContext(); got != "" {
		t.Errorf("want empty for missing config, got %q", got)
	}

	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"currentContext":"default"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := dockerHostFromContext(); got != "" {
		t.Errorf("want empty for default context, got %q", got)
	}
}
