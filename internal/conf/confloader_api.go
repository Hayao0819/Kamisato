package conf

import "github.com/spf13/pflag"

func Load[T any](
	dirs []string,
	files []string,
	flags *pflag.FlagSet,
	envPrefix string, // prefix for env var lookup (e.g. AYATO)
) (*T, error) {
	loader := New[T](".").
		Dirs(dirs...).
		Files(files...).
		PFlags(flags).
		Env(envPrefix)

	err := loader.Load()
	if err != nil {
		return nil, err
	}

	return loader.Get()
}

// Validator is implemented by config types that self-check after loading.
type Validator interface {
	Validate() error
}

// LoadTyped is Load plus the load-time skeleton every typed config shares:
// optional post-load defaults, then Validate. PT is *T so Validate, a
// pointer-receiver method, is reachable from the loaded value.
func LoadTyped[T any, PT interface {
	*T
	Validator
}](
	dirs []string,
	files []string,
	flags *pflag.FlagSet,
	envPrefix string,
	defaults func(PT),
) (*T, error) {
	cfg, err := Load[T](dirs, files, flags, envPrefix)
	if err != nil {
		return nil, err
	}
	if defaults != nil {
		defaults(PT(cfg))
	}
	if err := PT(cfg).Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}
