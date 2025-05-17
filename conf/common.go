package conf

import (
	"os"

	"github.com/Hayao0819/Kamisato/internal/kfutils"
)

func commonConfigDirs() []string {
	pwd, _ := os.Getwd()
	cfgdir, _ := os.UserConfigDir()

	dirs := []string{}
	if pwd != "" {
		dirs = append(dirs, pwd)
	}
	if cfgdir != "" {
		dirs = append(dirs, cfgdir)
	}
	return dirs
}

func loadConfig[T any](files ...string) (*T, error) {
	// l, err := kfutils.New[T](".").Dirs(commonConfigDirs()...).Files(files...).Load()
	// if err != nil {
	// 	return nil, err
	// }
	// return l.Get()

	return loadConfigWithDir[T](commonConfigDirs(), files)
}

func loadConfigWithDir[T any](dir []string, files []string) (*T, error) {
	l, err := kfutils.New[T](".").Dirs(dir...).Files(files...).Load()
	if err != nil {
		return nil, err
	}
	return l.Get()
}
