package conf

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
)

func TestMikoBuilderHostConfigMigratesLegacyFields(t *testing.T) {
	cfg := &MikoConfig{
		Executor:          "container",
		ArchBuildTemplate: "custom-%s-build",
		DockerHost:        "unix:///run/user/1000/docker.sock",
		Build: BuildServiceConfig{
			Image:      "archlinux:test",
			Timeout:    42,
			ExtraRepos: []builder.PacmanRepository{{Name: "ayato", Server: "https://repo/$repo/$arch"}},
		},
	}
	cfg.Cache.Enabled = true
	cfg.Cache.PacmanCacheDir = "/cache/pacman"
	cfg.Cache.CcacheDir = "/cache/ccache"

	got := cfg.BuilderHostConfig()
	if got.Backend != builder.KindContainer || got.Timeout != 42*time.Minute {
		t.Errorf("backend/timeout = %q/%s", got.Backend, got.Timeout)
	}
	if got.Docker.Image != "archlinux:test" || got.Docker.Host != cfg.DockerHost {
		t.Errorf("Docker = %+v", got.Docker)
	}
	if got.Docker.PacmanCacheDir != "/cache/pacman" || got.Bwrap.PacmanCacheDir != "/cache/pacman" || got.Docker.CcacheDir != "/cache/ccache" {
		t.Errorf("cache migration failed: docker=%+v bwrap=%+v", got.Docker, got.Bwrap)
	}
	if got.Devtools.ArchBuildTemplate != "custom-%s-build" || len(got.Repositories) != 1 {
		t.Errorf("template/repos migration failed: %+v", got)
	}
}

func TestMikoBuilderHostConfigPrefersNestedFields(t *testing.T) {
	cfg := &MikoConfig{
		Builder: builder.HostConfig{
			Backend: builder.KindChroot,
			Timeout: time.Hour,
			Docker:  builder.DockerConfig{Image: "new", Host: "unix:///new.sock"},
		},
		Executor:   "container",
		DockerHost: "unix:///legacy.sock",
		Build:      BuildServiceConfig{Image: "legacy", Timeout: 30},
	}
	got := cfg.BuilderHostConfig()
	if got.Backend != builder.KindChroot || got.Timeout != time.Hour || got.Docker.Image != "new" || got.Docker.Host != "unix:///new.sock" {
		t.Errorf("legacy fields overrode builder.*: %+v", got)
	}
}

func TestLoadMikoNestedBuilderConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "miko.yaml")
	data := []byte(`
builder:
  backend: bwrap
  timeout: 45m
  repositories:
    - name: ayato
      server: https://repo.example/$repo/$arch
  bwrap:
    rootfs: /srv/arch-rootfs
    pacman_cache_dir: /var/cache/miko/pacman
  docker:
    host: unix:///run/docker.sock
  devtools:
    archbuild_template: extra-%s-build
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadMikoConfig(nil, path)
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.BuilderHostConfig()
	if got.Backend != builder.KindBwrap || got.Timeout != 45*time.Minute || got.Bwrap.Rootfs != "/srv/arch-rootfs" {
		t.Errorf("loaded Builder = %+v", got)
	}
	if len(got.Repositories) != 1 || got.Repositories[0].Name != "ayato" {
		t.Errorf("loaded repositories = %+v", got.Repositories)
	}
}

func TestLoadMikoLegacyEnvOverridesCanonicalFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "miko.yaml")
	data := []byte(`
builder:
  backend: chroot
  timeout: 1h
  docker:
    image: file-image
    host: unix:///file.sock
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MIKO_EXECUTOR", "container")
	t.Setenv("MIKO_BUILD_TIMEOUT", "30")
	t.Setenv("MIKO_BUILD_IMAGE", "env-image")
	t.Setenv("MIKO_DOCKER_HOST", "unix:///env.sock")
	t.Setenv("MIKO_BUILD_EXTRA_REPOS", `[{"name":"envrepo","server":"https://env/$repo/$arch"}]`)
	t.Setenv("MIKO_CACHE_ENABLED", "true")
	t.Setenv("MIKO_CACHE_PACMAN_CACHE_DIR", "/env/pacman")
	t.Setenv("MIKO_CACHE_CCACHE_DIR", "/env/ccache")

	cfg, err := LoadMikoConfig(nil, path)
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.BuilderHostConfig()
	if got.Backend != builder.KindContainer || cfg.Executor != "container" {
		t.Errorf("backend/log mirror = %q/%q", got.Backend, cfg.Executor)
	}
	if got.Timeout != 30*time.Minute || got.Docker.Image != "env-image" || got.Docker.Host != "unix:///env.sock" {
		t.Errorf("legacy env did not override file: %+v", got)
	}
	if len(got.Repositories) != 1 || got.Repositories[0].Name != "envrepo" {
		t.Errorf("legacy env repositories = %+v", got.Repositories)
	}
	if got.Docker.PacmanCacheDir != "/env/pacman" || got.Bwrap.PacmanCacheDir != "/env/pacman" || got.Docker.CcacheDir != "/env/ccache" {
		t.Errorf("legacy env cache migration = docker %+v bwrap %+v", got.Docker, got.Bwrap)
	}
}

func TestLoadMikoCanonicalEnvWinsLegacyEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "miko.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MIKO_EXECUTOR", "chroot")
	t.Setenv("MIKO_BUILDER_BACKEND", "container")

	cfg, err := LoadMikoConfig(nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.BuilderHostConfig().Backend; got != builder.KindContainer || cfg.Executor != "container" {
		t.Errorf("canonical env did not win: backend=%q executor=%q", got, cfg.Executor)
	}
}

func TestMikoLegacyKeyInHigherPrecedenceFileWins(t *testing.T) {
	local := t.TempDir()
	global := t.TempDir()
	const filename = "miko.json"
	if err := os.WriteFile(filepath.Join(global, filename), []byte(`{"builder":{"backend":"chroot"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, filename), []byte(`{"executor":"container"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	loader := New[MikoConfig](".").
		Dirs(local, global).
		Files(filename).
		transformSources(migrateMikoBuilderSource)
	if err := loader.Load(); err != nil {
		t.Fatal(err)
	}
	cfg, err := loader.Get()
	if err != nil {
		t.Fatal(err)
	}
	cfg.applyDefaults()
	if got := cfg.BuilderHostConfig().Backend; got != builder.KindContainer {
		t.Errorf("higher-precedence legacy key lost to lower canonical key: %q", got)
	}
}

func TestLoadMikoRejectsUnitlessBuilderTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "miko.json")
	if err := os.WriteFile(path, []byte(`{"builder":{"timeout":30}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMikoConfig(nil, path); err == nil {
		t.Fatal("unitless builder.timeout: want error")
	}
}
