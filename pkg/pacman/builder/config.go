package builder

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
)

const (
	DefaultDockerImage = "archlinux:latest"
	// DefaultDockerTimeout preserves the container backend's 30-minute default.
	DefaultDockerTimeout = 30 * time.Minute
)

type PacmanRepository struct {
	Name     string `koanf:"name" json:"name"`
	Server   string `koanf:"server" json:"server"`
	SigLevel string `koanf:"siglevel" json:"siglevel,omitempty"`
}

type MakepkgConfig struct {
	Packager     string   `koanf:"packager" json:"packager,omitempty"`
	Microarch    string   `koanf:"microarch" json:"microarch,omitempty"`
	CFlagsAppend string   `koanf:"cflags_append" json:"cflags_append,omitempty"`
	Options      []string `koanf:"options" json:"options,omitempty"`
	// CompressZst overrides COMPRESSZST (e.g. "zstd -c -T0 --ultra -20 -"):
	// build images may default to zstd's weak level 3, which pushes a kernel
	// package past proxy upload limits.
	CompressZst string `koanf:"compresszst" json:"compresszst,omitempty"`
}

func (c MakepkgConfig) IsZero() bool {
	return c.Packager == "" && c.Microarch == "" && c.CFlagsAppend == "" && len(c.Options) == 0 && c.CompressZst == ""
}

// ProjectConfig omits host paths and daemon endpoints because repo.json is not trusted host configuration.
type ProjectConfig struct {
	Repos   []PacmanRepository `koanf:"repos" json:"repos,omitempty"`
	Makepkg MakepkgConfig      `koanf:"makepkg" json:"makepkg,omitempty"`
	Arches  []string           `koanf:"arches" json:"arches,omitempty"`
	Image   string             `koanf:"image" json:"image,omitempty"`
	// ArchBuild is read only to diagnose legacy repo.json files; Resolve ignores it.
	ArchBuild string `koanf:"archbuild" json:"archbuild,omitempty"`
	Timeout   string `koanf:"timeout" json:"timeout,omitempty"`
}

type DockerConfig struct {
	Image          string `koanf:"image" json:"image,omitempty"`
	Host           string `koanf:"host" json:"host,omitempty"`
	PacmanCacheDir string `koanf:"pacman_cache_dir" json:"pacman_cache_dir,omitempty"`
	CcacheDir      string `koanf:"ccache_dir" json:"ccache_dir,omitempty"`
}

type BwrapConfig struct {
	Rootfs         string `koanf:"rootfs" json:"rootfs,omitempty"`
	PacmanCacheDir string `koanf:"pacman_cache_dir" json:"pacman_cache_dir,omitempty"`
}

type DevtoolsConfig struct {
	ArchBuild string `koanf:"archbuild" json:"archbuild,omitempty"`
	// Resolve expands ArchBuildTemplate with the target arch when ArchBuild is empty.
	ArchBuildTemplate string `koanf:"archbuild_template" json:"archbuild_template,omitempty"`
}

// HostConfig is trusted operator configuration, including host paths and endpoints.
type HostConfig struct {
	Backend      Kind               `koanf:"backend" json:"backend,omitempty"`
	Timeout      time.Duration      `koanf:"timeout" json:"timeout,omitempty"`
	Repositories []PacmanRepository `koanf:"repositories" json:"repositories,omitempty"`
	Docker       DockerConfig       `koanf:"docker" json:"docker,omitempty"`
	Bwrap        BwrapConfig        `koanf:"bwrap" json:"bwrap,omitempty"`
	Devtools     DevtoolsConfig     `koanf:"devtools" json:"devtools,omitempty"`
}

type hostConfigJSON struct {
	Backend      Kind               `json:"backend,omitempty"`
	Timeout      string             `json:"timeout,omitempty"`
	Repositories []PacmanRepository `json:"repositories,omitempty"`
	Docker       DockerConfig       `json:"docker,omitempty"`
	Bwrap        BwrapConfig        `json:"bwrap,omitempty"`
	Devtools     DevtoolsConfig     `json:"devtools,omitempty"`
}

// MarshalJSON avoids time.Duration's unitless nanosecond representation.
func (host HostConfig) MarshalJSON() ([]byte, error) {
	var timeout string
	if host.Timeout != 0 {
		timeout = host.Timeout.String()
	}
	return json.Marshal(hostConfigJSON{
		Backend:      host.Backend,
		Timeout:      timeout,
		Repositories: host.Repositories,
		Docker:       host.Docker,
		Bwrap:        host.Bwrap,
		Devtools:     host.Devtools,
	})
}

// UnmarshalJSON requires units so a legacy 30-minute value cannot become 30ns.
func (host *HostConfig) UnmarshalJSON(data []byte) error {
	var decoded hostConfigJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var timeout time.Duration
	if decoded.Timeout != "" {
		parsed, err := time.ParseDuration(decoded.Timeout)
		if err != nil {
			return fmt.Errorf("builder.timeout %q is not a valid duration: %w", decoded.Timeout, err)
		}
		timeout = parsed
	}
	*host = HostConfig{
		Backend:      decoded.Backend,
		Timeout:      timeout,
		Repositories: decoded.Repositories,
		Docker:       decoded.Docker,
		Bwrap:        decoded.Bwrap,
		Devtools:     decoded.Devtools,
	}
	return nil
}

// BuildOverrides excludes host paths and endpoints from per-project control.
type BuildOverrides struct {
	Timeout      time.Duration
	Repositories []PacmanRepository
	Makepkg      MakepkgConfig
	DockerImage  string
}

// ResolvedConfig is the backend input produced by Resolve.
type ResolvedConfig struct {
	Backend      Kind
	Timeout      time.Duration
	Repositories []PacmanRepository
	Makepkg      MakepkgConfig
	Docker       DockerConfig
	Bwrap        BwrapConfig
	Devtools     DevtoolsConfig
}

func (c ProjectConfig) Overrides(arch string) (BuildOverrides, error) {
	if len(c.Arches) > 0 && !slices.Contains(c.Arches, arch) {
		return BuildOverrides{}, fmt.Errorf("arch %s is not in build.arches (%s)", arch, strings.Join(c.Arches, ","))
	}
	if err := ValidateRepositories(c.Repos); err != nil {
		return BuildOverrides{}, fmt.Errorf("build.repos: %w", err)
	}
	var timeout time.Duration
	if c.Timeout != "" {
		parsed, err := time.ParseDuration(c.Timeout)
		if err != nil {
			return BuildOverrides{}, fmt.Errorf("build.timeout %s is not a valid duration: %w", c.Timeout, err)
		}
		timeout = parsed
	}
	return BuildOverrides{
		Timeout:      timeout,
		Repositories: cloneRepositories(c.Repos),
		Makepkg:      cloneMakepkg(c.Makepkg),
		DockerImage:  c.Image,
	}, nil
}

// Resolve applies restricted build overrides without exposing host-only settings.
// Host repositories retain precedence by appearing first in pacman.conf.
func Resolve(host HostConfig, overrides BuildOverrides, arch string) (ResolvedConfig, error) {
	if err := host.Validate(); err != nil {
		return ResolvedConfig{}, err
	}
	backend := host.Backend
	if backend == "" {
		backend = KindChroot
	}
	if !ValidKind(backend) {
		return ResolvedConfig{}, fmt.Errorf("unknown build backend %q", backend)
	}
	if overrides.Timeout < 0 {
		return ResolvedConfig{}, fmt.Errorf("build timeout must not be negative")
	}
	if !ValidMicroarch(overrides.Makepkg.Microarch) {
		return ResolvedConfig{}, fmt.Errorf("unknown microarchitecture tier %q", overrides.Makepkg.Microarch)
	}
	if overrides.Makepkg.Microarch != "" && arch != "x86_64" {
		return ResolvedConfig{}, fmt.Errorf("microarchitecture tier %q requires target arch x86_64, got %q", overrides.Makepkg.Microarch, arch)
	}

	timeout := host.Timeout
	if overrides.Timeout > 0 {
		timeout = overrides.Timeout
	}
	if timeout == 0 && backend == KindContainer {
		timeout = DefaultDockerTimeout
	}
	dockerConfig := host.Docker
	if overrides.DockerImage != "" {
		dockerConfig.Image = overrides.DockerImage
	}
	dockerConfig.Image = strings.ReplaceAll(dockerConfig.Image, "$arch", arch)
	if dockerConfig.Image == "" && backend == KindContainer {
		dockerConfig.Image = DefaultDockerImage
	}

	devtoolsConfig := host.Devtools
	if devtoolsConfig.ArchBuild == "" && devtoolsConfig.ArchBuildTemplate != "" {
		devtoolsConfig.ArchBuild = formatArchBuild(devtoolsConfig.ArchBuildTemplate, arch)
	}

	repositories := append(cloneRepositories(host.Repositories), overrides.Repositories...)
	if err := ValidateRepositories(repositories); err != nil {
		return ResolvedConfig{}, fmt.Errorf("build repositories: %w", err)
	}
	resolved := ResolvedConfig{
		Backend:      backend,
		Timeout:      timeout,
		Repositories: repositories,
		Makepkg:      cloneMakepkg(overrides.Makepkg),
		Docker:       dockerConfig,
		Bwrap:        host.Bwrap,
		Devtools:     devtoolsConfig,
	}
	if err := resolved.Validate(); err != nil {
		return ResolvedConfig{}, err
	}
	return resolved, nil
}

func (host HostConfig) Validate() error {
	if host.Backend != "" && !ValidKind(host.Backend) {
		return fmt.Errorf("unknown build backend %q", host.Backend)
	}
	if host.Timeout < 0 {
		return fmt.Errorf("builder timeout must not be negative")
	}
	if err := ValidateRepositories(host.Repositories); err != nil {
		return fmt.Errorf("builder.repositories: %w", err)
	}
	if host.Backend == KindBwrap && host.Bwrap.Rootfs == "" {
		return fmt.Errorf("bwrap backend requires builder.bwrap.rootfs")
	}
	return nil
}

func (config ResolvedConfig) Validate() error {
	if !ValidKind(config.Backend) {
		return fmt.Errorf("unknown build backend %q", config.Backend)
	}
	if config.Timeout < 0 {
		return fmt.Errorf("build timeout must not be negative")
	}
	if err := ValidateRepositories(config.Repositories); err != nil {
		return fmt.Errorf("build repositories: %w", err)
	}
	if !ValidMicroarch(config.Makepkg.Microarch) {
		return fmt.Errorf("unknown microarchitecture tier %q", config.Makepkg.Microarch)
	}
	switch config.Backend {
	case KindContainer:
		if config.Docker.Image == "" {
			return fmt.Errorf("container backend requires a resolved Docker image")
		}
		if config.Timeout == 0 {
			return fmt.Errorf("container backend requires a resolved timeout")
		}
	case KindBwrap:
		if config.Bwrap.Rootfs == "" {
			return fmt.Errorf("bwrap backend requires builder.bwrap.rootfs")
		}
	case KindChroot:
		if len(config.Repositories) == 0 && config.Makepkg.IsZero() && config.Devtools.ArchBuild == "" {
			return fmt.Errorf("chroot backend requires builder.devtools.archbuild or generated build configuration")
		}
	}
	return nil
}

// ValidateRepositories prevents config injection and trusted-repository shadowing.
func ValidateRepositories(repos []PacmanRepository) error {
	seen := make(map[string]struct{}, len(repos))
	for i, repo := range repos {
		if repo.Name == "" {
			return fmt.Errorf("repository %d: name is required", i)
		}
		if repo.Server == "" {
			return fmt.Errorf("repository %q: server is required", repo.Name)
		}
		if !validRepositoryName(repo.Name) {
			return fmt.Errorf("repository %d: invalid name %q", i, repo.Name)
		}
		if strings.IndexFunc(repo.Server, unicode.IsControl) >= 0 {
			return fmt.Errorf("repository %q: server contains a control character", repo.Name)
		}
		if strings.IndexFunc(repo.SigLevel, unicode.IsControl) >= 0 {
			return fmt.Errorf("repository %q: siglevel contains a control character", repo.Name)
		}
		if _, ok := seen[repo.Name]; ok {
			return fmt.Errorf("duplicate repository name %q", repo.Name)
		}
		seen[repo.Name] = struct{}{}
	}
	return nil
}

func validRepositoryName(name string) bool {
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("._@+-", r) {
			continue
		}
		return false
	}
	return name != ""
}

func ValidKind(kind Kind) bool {
	switch kind {
	case KindChroot, KindContainer, KindBwrap:
		return true
	default:
		return false
	}
}

func formatArchBuild(template, arch string) string {
	if strings.Contains(template, "%s") {
		return fmt.Sprintf(template, arch)
	}
	return template
}

func cloneRepositories(in []PacmanRepository) []PacmanRepository {
	return append([]PacmanRepository(nil), in...)
}

func cloneMakepkg(in MakepkgConfig) MakepkgConfig {
	in.Options = append([]string(nil), in.Options...)
	return in
}
