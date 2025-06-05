package kfutils

import (
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
)

type Loader[T any] struct {
	k            *koanf.Koanf
	dirs         []string
	filenames    []string
	envPrefix    string
	envDelimiter string
	envKeyMap    func(string) string
	pflags       *pflag.FlagSet
}

func (l *Loader[T]) Get() (*T, error) {
	var cfg T
	err := l.Unmarshal(&cfg)
	return &cfg, err
}

func (l *Loader[T]) Unmarshal(v *T) error {
	return l.k.Unmarshal("", v)
}

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
	envPrefix string, // 環境変数読み込み時のプレフィックス（例: APP_）
	envDelimiter string, // 環境変数の区切り（例: .）
	envKeyMap func(string) string, // 任意のキー変換関数
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
