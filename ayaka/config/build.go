package config

import (
	"fmt"
	"os"
	"path/filepath"

	configloader "github.com/Hayao0819/Kamisato/internal/config"
	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
	"github.com/knadh/koanf/v2"
)

type directBuildHostFile struct {
	Builder builder.HostConfig `koanf:"builder"`
}

func (c *directBuildHostFile) Validate() error {
	return c.Builder.Validate()
}

// LoadDirectBuildHostConfig reads only an explicitly supplied configuration;
// direct package builds must not discover local source repository settings.
func LoadDirectBuildHostConfig(path string) (builder.HostConfig, error) {
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
