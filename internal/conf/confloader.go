// Package conf defines each binary's configuration structs and loads them with a
// thin koanf wrapper that merges multiple directories, multiple formats
// (JSON/TOML/YAML), environment variables and pflag.
package conf

import (
	encjson "encoding/json"
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
	k                *koanf.Koanf
	dirs             []string
	filenames        []string
	envPrefix        string
	pflags           *pflag.FlagSet
	sourceTransforms []sourceTransform
}

// sourceTransform runs before merging so aliases keep provider precedence.
type sourceTransform func(*koanf.Koanf) error

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

func (l *Loader[T]) transformSources(transforms ...sourceTransform) *Loader[T] {
	l.sourceTransforms = append(l.sourceTransforms, transforms...)
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
		revMap, err := envPathMap(reflect.TypeFor[T](), l.envPrefix)
		if err != nil {
			return err
		}
		// The transform callback cannot return an error, so the first parse
		// failure is stashed here and surfaced once Load returns.
		var parseErr error
		provider := env.Provider(".", env.Opt{
			Prefix: strings.ToUpper(l.envPrefix) + "_",
			TransformFunc: func(k, v string) (string, any) {
				leaf, ok := revMap[k]
				if !ok {
					return "", nil
				}
				val, perr := parseEnvValue(leaf.typ, v)
				if perr != nil {
					if parseErr == nil {
						parseErr = fmt.Errorf("env %s: %w", k, perr)
					}
					return "", nil
				}
				return leaf.path, val
			},
		})
		if err := l.loadSource(provider, nil); err != nil {
			return fmt.Errorf("failed to load env vars: %w", err)
		}
		if parseErr != nil {
			return parseErr
		}
	}

	if l.pflags != nil {
		// Flag names use hyphens (CLI convention) but koanf keys use underscores,
		// so normalize the key (e.g. --ayato-url -> ayato_url). FlagVal keeps the
		// flag's typed value; the nil koanf means only user-set flags are merged.
		provider := posflag.ProviderWithFlag(l.pflags, ".", nil, func(f *pflag.Flag) (string, any) {
			return strings.ReplaceAll(f.Name, "-", "_"), posflag.FlagVal(l.pflags, f)
		})
		if err := l.loadSource(provider, nil); err != nil {
			return fmt.Errorf("failed to load pflags: %w", err)
		}
	}

	return nil
}

// loadFile merges one config file. A missing file is skipped silently; an
// unsupported extension is an error (a likely typo that would otherwise leave the
// program running on empty defaults).
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
	if err := l.loadSource(file.Provider(path), parser); err != nil {
		return fmt.Errorf("failed to load %s: %w", path, err)
	}
	return nil
}

func (l *Loader[T]) loadSource(provider koanf.Provider, parser koanf.Parser) error {
	source := koanf.New(l.k.Delim())
	if err := source.Load(provider, parser); err != nil {
		return err
	}
	for _, transform := range l.sourceTransforms {
		if err := transform(source); err != nil {
			return err
		}
	}
	return l.k.Merge(source)
}

func (l *Loader[T]) Get() (*T, error) {
	var cfg T
	err := l.Unmarshal(&cfg)
	return &cfg, err
}

func (l *Loader[T]) Unmarshal(v *T) error {
	return l.k.Unmarshal("", v)
}

// envLeaf pairs a koanf dotted path with the Go type of the field it targets, so
// the env transform knows whether a value is a scalar or a slice/map to parse.
type envLeaf struct {
	path string
	typ  reflect.Type
}

// envPathMap maps each leaf's env name to its koanf path and field type, derived
// from the koanf struct tags of T. The env name is the prefix plus the uppercased
// path with dots replaced by underscores, e.g. auth.session_secret ->
// AYATO_AUTH_SESSION_SECRET.
func envPathMap(t reflect.Type, prefix string) (map[string]envLeaf, error) {
	var leaves []envLeaf
	collectKoanfPaths(t, "", &leaves)

	rev := make(map[string]envLeaf, len(leaves))
	for _, lf := range leaves {
		name := strings.ToUpper(prefix + "_" + strings.ReplaceAll(lf.path, ".", "_"))
		if other, ok := rev[name]; ok {
			return nil, fmt.Errorf("env name %q maps to both %q and %q", name, other.path, lf.path)
		}
		rev[name] = lf
	}
	return rev, nil
}

// collectKoanfPaths walks t's koanf tags, recursing into nested structs and
// treating slices and maps as single leaves.
func collectKoanfPaths(t reflect.Type, prefix string, out *[]envLeaf) {
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
		*out = append(*out, envLeaf{path: path, typ: f.Type})
	}
}

// parseEnvValue shapes an env var's string value for the target field. Scalars
// pass through unchanged (mapstructure coerces them on Unmarshal). Slice, array
// and map fields — which koanf's single-env-per-key model otherwise can't fill —
// accept a JSON literal; a slice of scalars additionally accepts a plain
// comma-separated list for the common case (e.g. trusted_proxies=a,b,c).
func parseEnvValue(typ reflect.Type, v string) (any, error) {
	for typ != nil && typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ == nil {
		return v, nil
	}
	switch typ.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		return parseStructuredEnv(typ, v)
	default:
		return v, nil
	}
}

func parseStructuredEnv(typ reflect.Type, v string) (any, error) {
	trimmed := strings.TrimSpace(v)
	if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{") {
		var out any
		if err := encjson.Unmarshal([]byte(trimmed), &out); err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
		return out, nil
	}
	// Only a slice/array of scalar elements can take the bare comma form; a map or
	// a slice of structs is ambiguous, so require JSON there.
	if typ.Kind() == reflect.Map {
		return nil, fmt.Errorf("value for a %s must be a JSON object (starting with '{')", typ)
	}
	elem := typ.Elem()
	for elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	if !isScalarKind(elem.Kind()) {
		return nil, fmt.Errorf("value for a %s must be a JSON array (starting with '[')", typ)
	}
	if trimmed == "" {
		return []string{}, nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = strings.TrimSpace(p)
	}
	return out, nil
}

func isScalarKind(k reflect.Kind) bool {
	switch k {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
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
