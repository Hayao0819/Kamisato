package confloader

import "github.com/spf13/pflag"

func SimpleLoad[T any](dir []string, files []string) (*T, error) {
	l := New[T](".").Dirs(dir...).Files(files...)
	if err := l.Load(); err != nil {
		return nil, err
	}
	return l.Get()
}

func Load[T any](
	dirs []string,
	files []string,
	flags *pflag.FlagSet,
	envPrefix string, // prefix for env var lookup (e.g. APP_)
	envDelimiter string, // env var delimiter (e.g. .)
	envKeyMap func(string) string, // optional key transform function
) (*T, error) {
	loader := New[T](".").
		Dirs(dirs...).
		Files(files...).
		PFlags(flags).
		Env(envPrefix, envDelimiter, envKeyMap)

	err := loader.Load()
	if err != nil {
		return nil, err
	}

	return loader.Get()
}
