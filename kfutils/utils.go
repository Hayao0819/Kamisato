package kfutils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Loader[T any] struct {
	k *koanf.Koanf
}

// LoadFiles loads and merges config files found in the given directories and filenames
func (l *Loader[T]) LoadFiles(dirs []string, filenames []string) error {
	for _, dir := range dirs {
		for _, filename := range filenames {
			path := filepath.Join(dir, filename)

			if _, err := os.Stat(path); err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return fmt.Errorf("failed to stat %s: %w", path, err)
			}

			ext := filepath.Ext(path)
			parser, err := parserForExtension(ext)
			if err != nil {
				fmt.Printf("Skipping unsupported file: %s\n", path)
				continue
			}

			fmt.Printf("Loading config (%s): %s\n", ext, path)
			if err := l.k.Load(file.Provider(path), parser); err != nil {
				return fmt.Errorf("failed to load %s: %w", path, err)
			}
		}
	}
	return nil
}

type UnmarshalOptions struct {
	Tag string // struct tag (default "koanf")
}

// Unmarshal maps the loaded config into any struct type (generic)
func (l *Loader[T]) Unmarshal(opts ...UnmarshalOptions) (T, error) {
	var cfg T
	tag := "koanf"
	if len(opts) > 0 && opts[0].Tag != "" {
		tag = opts[0].Tag
	}
	err := l.k.Unmarshal(tag, &cfg)
	return cfg, err
}
