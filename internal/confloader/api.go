package confloader

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
