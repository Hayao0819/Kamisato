package conf

import (
	"log/slog"

	"github.com/spf13/pflag"
)

type AyatoConfig struct {
	Debug   bool            `koanf:"debug"`
	Repos   []BinRepoConfig `koanf:"repos"`
	Port    int             `koanf:"port"`
	MaxSize int             `koanf:"maxsize"`
	Auth    AuthConfig      `koanf:"auth"`
	Store   StoreConfig     `koanf:"store"`
}

type AuthConfig struct {
	Username string `koanf:"username"`
	Password string `koanf:"password"`
}

type BinRepoConfig struct {
	Name   string   `koanf:"name"`
	Arches []string `koanf:"arches"`
}

func LoadAyatoConfig(flags *pflag.FlagSet) (*AyatoConfig, error) {
	// return loadConfig[AyatoConfig]("ayato_config.json")

	if err := LoadEnv(); err != nil {
		slog.Error("Failed to load env", "error", err)
	}

	return loadConfig[AyatoConfig](
		commonConfigDirs(),
		[]string{"ayato_config.json", "ayato_config.toml", "ayato_config.yaml"},
		flags,
		"AYATO",
	)

}
