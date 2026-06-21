// Package confloader is a thin wrapper around koanf that loads config by merging
// multiple directories, multiple formats (JSON/TOML/YAML), environment variables
// and pflag.
package confloader

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
)

// Loader is a builder that assembles search conditions incrementally and loads config.
type Loader[T any] struct {
	k            *koanf.Koanf
	dirs         []string
	filenames    []string
	envPrefix    string
	envDelimiter string
	envKeyMap    func(string) string
	pflags       *pflag.FlagSet
}

// New returns a new Loader instance with a custom delimiter (default ".")
func New[T any](delimiter string) *Loader[T] {
	if delimiter == "" {
		delimiter = "."
	}
	return &Loader[T]{k: koanf.New(delimiter)}
}

func (l *Loader[T]) Dirs(dirs ...string) *Loader[T] {
	l.dirs = dirs
	return l
}

func (l *Loader[T]) Files(filenames ...string) *Loader[T] {
	l.filenames = filenames
	return l
}

func (l *Loader[T]) Env(prefix, delimiter string, keyMap func(string) string) *Loader[T] {
	l.envPrefix = prefix
	l.envDelimiter = delimiter
	l.envKeyMap = keyMap

	return l
}

func (l *Loader[T]) PFlags(flags *pflag.FlagSet) *Loader[T] {
	l.pflags = flags
	return l
}

// Load loads and merges config files based on Dirs() and Files() state
func (l *Loader[T]) Load() error {
	// An absolute filename is used as-is; a relative one is searched under each
	// dir. Joining an absolute path with a dir would mangle it.
	for _, filename := range l.filenames {
		if filepath.IsAbs(filename) {
			if err := l.loadFile(filename); err != nil {
				return err
			}
			continue
		}
		for _, dir := range l.dirs {
			if err := l.loadFile(filepath.Join(dir, filename)); err != nil {
				return err
			}
		}
	}

	if l.envPrefix != "" {
		if err := l.k.Load(env.Provider(l.envPrefix, l.envDelimiter, l.envKeyMap), nil); err != nil {
			return fmt.Errorf("failed to load env vars: %w", err)
		}
	}

	if l.pflags != nil {
		if err := l.k.Load(posflag.Provider(l.pflags, ".", nil), nil); err != nil {
			return fmt.Errorf("failed to load pflags: %w", err)
		}
	}

	return nil
}

// loadFile merges one config file. A missing file or unsupported extension is
// skipped silently; other errors are returned.
func (l *Loader[T]) loadFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to stat %s: %w", path, err)
	}
	if info.IsDir() {
		return nil
	}

	parser, err := parserForExtension(filepath.Ext(path))
	if err != nil {
		slog.Debug("Skipping unsupported file", "path", path, "ext", filepath.Ext(path))
		return nil
	}

	slog.Debug("Loading config", "path", path)
	if err := l.k.Load(file.Provider(path), parser); err != nil {
		return fmt.Errorf("failed to load %s: %w", path, err)
	}
	return nil
}

func (l *Loader[T]) Get() (*T, error) {
	var cfg T
	err := l.Unmarshal(&cfg)
	return &cfg, err
}

func (l *Loader[T]) Unmarshal(v *T) error {
	return l.k.Unmarshal("", v)
}

func parserForExtension(ext string) (koanf.Parser, error) {
	switch strings.ToLower(ext) {
	case ".yaml", ".yml":
		return yaml.Parser(), nil
	case ".json":
		return json.Parser(), nil
	case ".toml":
		return toml.Parser(), nil
	default:
		return nil, fmt.Errorf("unsupported file extension: %s", ext)
	}
}
