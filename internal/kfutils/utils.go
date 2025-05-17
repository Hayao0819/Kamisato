package kfutils

import (
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
)

type Loader[T any] struct {
	k         *koanf.Koanf
	dirs      []string
	filenames []string
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
	l, err := New[T](".").Dirs(dir...).Files(files...).Load()
	if err != nil {
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
	loader := New[T](".")

	// 環境変数のロード
	if envPrefix != "" {
		loader.WithEnv(envPrefix, envDelimiter, envKeyMap)
	}

	// フラグのロード
	if flags != nil {
		loader.WithPFlag(flags)
	}

	// ファイルのロード
	l, err := loader.Dirs(dirs...).Files(files...).Load()
	if err != nil {
		return nil, err
	}

	return l.Get()
}
