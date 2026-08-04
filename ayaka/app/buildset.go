package app

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	buildsetapp "github.com/Hayao0819/Kamisato/internal/buildset"
	configloader "github.com/Hayao0819/Kamisato/internal/config"
	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
	"github.com/Hayao0819/Kamisato/internal/pacman/builder/factory"
	"github.com/knadh/koanf/v2"
)

const defaultDirectBuildImage = "ghcr.io/hayao0819/archlinux:$arch"

type directBuildHostFile struct {
	Builder builder.HostConfig `koanf:"builder"`
}

func (c *directBuildHostFile) Validate() error {
	return c.Builder.Validate()
}

type DirectBuildOptions struct {
	ConfigFile string
	Arch       string
	Backend    builder.Kind
	Image      string
	Timeout    time.Duration
	Makepkg    builder.MakepkgConfig
}

func NewDirectBuildApplication(options DirectBuildOptions) (*buildsetapp.Application, builder.BuildEnvironment, error) {
	host, err := loadDirectBuildHostConfig(options.ConfigFile)
	if err != nil {
		return nil, builder.BuildEnvironment{}, err
	}
	if len(host.Repositories) > 0 {
		return nil, builder.BuildEnvironment{}, fmt.Errorf("builder.repositories cannot be used by direct package builds; configure repositories in --pacman-conf")
	}
	if options.Backend != "" {
		host.Backend = options.Backend
	}
	if host.Backend == "" {
		host.Backend = builder.KindContainer
	}
	if host.Backend != builder.KindContainer {
		return nil, builder.BuildEnvironment{}, fmt.Errorf("direct package builds currently require the container backend because chroot tooling evaluates PKGBUILD on the host")
	}
	image := options.Image
	if image == "" && host.Docker.Image == "" {
		image = defaultDirectBuildImage
	}
	overrides := builder.BuildOverrides{DockerImage: image, Timeout: options.Timeout, Makepkg: options.Makepkg}
	resolved, err := builder.Resolve(host, overrides, options.Arch)
	if err != nil {
		return nil, builder.BuildEnvironment{}, fmt.Errorf("resolve direct build backend: %w", err)
	}
	backend, err := factory.New(resolved)
	if err != nil {
		return nil, builder.BuildEnvironment{}, err
	}
	application := buildsetapp.New(buildsetapp.Dependencies{Backend: backend})
	return application, builder.BuildEnvironment{Image: resolved.Docker.Image}, nil
}

func loadDirectBuildHostConfig(path string) (builder.HostConfig, error) {
	if path == "" {
		return builder.HostConfig{}, nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return builder.HostConfig{}, err
	}
	if _, err := os.Stat(abs); err != nil {
		return builder.HostConfig{}, fmt.Errorf("read direct build config %q: %w", abs, err)
	}
	loaded, err := configloader.LoadTypedWithSourceTransforms[directBuildHostFile](
		nil,
		[]string{abs},
		nil,
		"",
		nil,
		func(source *koanf.Koanf) error {
			return configloader.ValidateDuration(source, "builder.timeout")
		},
	)
	if err != nil {
		return builder.HostConfig{}, err
	}
	return loaded.Builder, nil
}
