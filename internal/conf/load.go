package conf

import (
	"os"

	"github.com/Hayao0819/Kamisato/internal/confloader"
	"github.com/spf13/pflag"
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

func loadConfig[T any](dirs []string, files []string, flags *pflag.FlagSet, envPrefix string) (*T, error) {
	return confloader.Load[T](dirs, files, flags, envPrefix)
}
