package kfutils

import (
	"fmt"
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

// Load loads and merges config files based on Dirs() and Files() state
func (l *Loader[T]) Load() (*Loader[T], error) {
	for _, dir := range l.dirs {
		for _, filename := range l.filenames {
			path := filepath.Join(dir, filename)

			info, err := os.Stat(path)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return l, fmt.Errorf("failed to stat %s: %w", path, err)
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

			fmt.Printf("Loading config (%s): %s\n", ext, path)
			if err := l.k.Load(file.Provider(path), parser); err != nil {
				return l, fmt.Errorf("failed to load %s: %w", path, err)
			}
		}
	}
	return l, nil
}

// LoadFiles is a wrapper for Dirs + Files + Load
func (l *Loader[T]) LoadFiles(dirs []string, filenames []string) (*Loader[T], error) {
	return l.Dirs(dirs...).Files(filenames...).Load()
}

// WithEnv loads configuration from environment variables
func (l *Loader[T]) WithEnv(prefix, delimiter string, keyMap func(string) string) *Loader[T] {
	l.k.Load(env.Provider(prefix, delimiter, keyMap), nil)
	return l
}

// WithPFlag loads configuration from pflag.FlagSet
func (l *Loader[T]) WithPFlag(fs *pflag.FlagSet) *Loader[T] {
	l.k.Load(posflag.Provider(fs, ".", l.k), nil)
	return l
}
