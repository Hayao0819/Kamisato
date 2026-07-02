// Package confloader is a thin wrapper around koanf that loads config by merging
// multiple directories, multiple formats (JSON/TOML/YAML), environment variables
// and pflag.
package confloader

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/parsers/yaml"
	env "github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
)

// Loader is a builder that assembles search conditions incrementally and loads config.
type Loader[T any] struct {
	k         *koanf.Koanf
	dirs      []string
	filenames []string
	envPrefix string
	pflags    *pflag.FlagSet
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

func (l *Loader[T]) Env(prefix string) *Loader[T] {
	l.envPrefix = prefix
	return l
}

func (l *Loader[T]) PFlags(flags *pflag.FlagSet) *Loader[T] {
	l.pflags = flags
	return l
}

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
		// Dirs are listed highest-precedence first (project-local before the global
		// user config dir), but koanf merges last-wins, so load them lowest-first.
		for _, dir := range slices.Backward(l.dirs) {
			if err := l.loadFile(filepath.Join(dir, filename)); err != nil {
				return err
			}
		}
	}

	if l.envPrefix != "" {
		// Env names stay pretty (single underscores). Map each one to its exact
		// dotted koanf path so multi-word tags and cross-section keys resolve.
		revMap, err := envPathMap(reflect.TypeOf((*T)(nil)).Elem(), l.envPrefix)
		if err != nil {
			return err
		}
		provider := env.Provider(".", env.Opt{
			Prefix: strings.ToUpper(l.envPrefix) + "_",
			TransformFunc: func(k, v string) (string, any) {
				if path, ok := revMap[k]; ok {
					return path, v
				}
				return "", nil
			},
		})
		if err := l.k.Load(provider, nil); err != nil {
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

// loadFile merges one config file. A missing file is skipped silently; a file
// that exists but has an unsupported extension is an error (a likely typo that
// would otherwise leave the program running on empty defaults). Parse errors are
// returned too.
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
		return fmt.Errorf("%s: %w", path, err)
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

// envPathMap maps each leaf's env name to its dotted koanf path, derived from the
// koanf struct tags of T. The env name is the prefix plus the uppercased path with
// dots replaced by underscores, e.g. auth.session_secret -> AYATO_AUTH_SESSION_SECRET.
func envPathMap(t reflect.Type, prefix string) (map[string]string, error) {
	var paths []string
	collectKoanfPaths(t, "", &paths)

	rev := make(map[string]string, len(paths))
	for _, p := range paths {
		name := strings.ToUpper(prefix + "_" + strings.ReplaceAll(p, ".", "_"))
		if other, ok := rev[name]; ok {
			return nil, fmt.Errorf("env name %q maps to both %q and %q", name, other, p)
		}
		rev[name] = p
	}
	return rev, nil
}

// collectKoanfPaths walks t's koanf tags, recursing into nested structs and
// treating slices and maps as single leaves.
func collectKoanfPaths(t reflect.Type, prefix string, out *[]string) {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("koanf"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}

		ft := f.Type
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Struct {
			collectKoanfPaths(ft, path, out)
			continue
		}
		*out = append(*out, path)
	}
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
