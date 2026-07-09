package conf

import (
	"log/slog"
	"os"
	"path/filepath"

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
		// Prefer a dedicated subdir, but keep the bare config dir for back-compat.
		dirs = append(dirs, filepath.Join(cfgdir, "kamisato"), cfgdir)
	}
	return dirs
}

func loadConfig[T any](dirs []string, files []string, flags *pflag.FlagSet, envPrefix string) (*T, error) {
	return Load[T](dirs, files, flags, envPrefix)
}

// configFileNames returns the explicit config file when one is given, else the
// default <base>.{json,toml,yaml} search list the server loaders share.
func configFileNames(configFile, base string) []string {
	if configFile != "" {
		return []string{configFile}
	}
	return []string{base + ".json", base + ".toml", base + ".yaml"}
}

// loadDotEnv loads a .env file when present, logging (not failing) on error so
// every server loader picks up env-provided secrets the same way.
func loadDotEnv() {
	if err := LoadEnv(); err != nil {
		slog.Error("Failed to load env", "error", err)
	}
}
