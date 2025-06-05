package kfutils

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/spf13/pflag"
)

// Dirs sets search directories
func (l *Loader[T]) Dirs(dirs ...string) *Loader[T] {
	l.dirs = dirs
	return l
}

// Files sets filenames to search for
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
	// Load files from directories
	for _, dir := range l.dirs {
		for _, filename := range l.filenames {
			path := filepath.Join(dir, filename)

			info, err := os.Stat(path)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return fmt.Errorf("failed to stat %s: %w", path, err)
			}
			if info.IsDir() {
				continue
			}

			ext := filepath.Ext(path)
			parser, err := parserForExtension(ext)
			if err != nil {
				fmt.Printf("Skipping unsupported file: %s\n", path)
				continue
			}

			// fmt.Printf("Loading config (%s): %s\n", ext, path)
			slog.Debug("Loading config", "path", path)
			if err := l.k.Load(file.Provider(path), parser); err != nil {
				return fmt.Errorf("failed to load %s: %w", path, err)
			}
		}
	}

	// Load environment variables
	if l.envPrefix != "" {
		if err := l.k.Load(env.Provider(l.envPrefix, l.envDelimiter, l.envKeyMap), nil); err != nil {
			return fmt.Errorf("failed to load env vars: %w", err)
		}
	}

	// Load pflag values
	if l.pflags != nil {
		if err := l.k.Load(posflag.Provider(l.pflags, ".", nil), nil); err != nil {
			return fmt.Errorf("failed to load pflags: %w", err)
		}
	}

	return nil
}
