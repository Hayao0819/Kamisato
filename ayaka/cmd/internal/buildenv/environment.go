package buildenv

import (
	"fmt"
	"time"

	"github.com/Hayao0819/Kamisato/ayaka/service/buildset"
	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
	"github.com/Hayao0819/Kamisato/internal/pacman/builder/factory"
)

const defaultDirectBuildImage = "ghcr.io/hayao0819/archlinux:$arch"

type Options struct {
	Arch    string
	Backend builder.Kind
	Image   string
	Timeout time.Duration
	Makepkg builder.MakepkgConfig
}

// New resolves a backend from explicit host configuration and CLI overrides.
// Configuration loading belongs to the caller, not to the build dependencies.
func New(host builder.HostConfig, options Options) (*buildset.Service, builder.BuildEnvironment, error) {
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
	service := buildset.New(buildset.Dependencies{Backend: backend})
	return service, builder.BuildEnvironment{Image: resolved.Docker.Image}, nil
}
