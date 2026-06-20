package confloader

import "github.com/spf13/pflag"

// SimpleLoad はディレクトリ群とファイル名群からのみ設定をロードします。
func SimpleLoad[T any](dir []string, files []string) (*T, error) {
	l := New[T](".").Dirs(dir...).Files(files...)
	if err := l.Load(); err != nil {
		return nil, err
	}
	return l.Get()
}

// Load はファイル・環境変数・pflag を統合して設定をロードします。
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
