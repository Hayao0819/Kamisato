// Package factory selects concrete backends without adding their dependencies to builder.
package factory

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder/bwrap"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder/devtools"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder/docker"
)

func New(config builder.ResolvedConfig) (builder.Backend, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid resolved builder configuration: %w", err)
	}
	switch config.Backend {
	case builder.KindChroot:
		return devtools.New(config), nil
	case builder.KindContainer:
		return docker.New(config), nil
	case builder.KindBwrap:
		return bwrap.New(config), nil
	default:
		return nil, fmt.Errorf("unknown build backend %q", config.Backend)
	}
}
