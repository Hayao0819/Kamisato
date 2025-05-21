package conf

import (
	"log/slog"
	"path"

	"github.com/spf13/pflag"
)

type AyatoConfig struct {
	Debug    bool     `koanf:"debug"`
	RepoPath []string `koanf:"repopath"`
	Port     int      `koanf:"port"`
	DataPath string   `koanf:"datapath"`
	Username string   `koanf:"username"`
	Password string   `koanf:"password"`
	MaxSize  int      `koanf:"maxsize"`
	Database DbConfig `koanf:"dbconfig"`
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

func (c *AyatoConfig) DbPath() string {
	return path.Join(c.DataPath, "kv-db")
}
