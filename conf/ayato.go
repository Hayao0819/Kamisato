package conf

import (
	"path"

	"github.com/spf13/pflag"
)

type AyatoConfig struct {
	RepoPath []string `koanf:"repopath"`
	Port     int      `koanf:"port"`
	DataPath string   `koanf:"datapath"`
	Username string   `koanf:"username"`
	Password string   `koanf:"password"`
	MaxSize  int      `koanf:"maxsize"`
}

func LoadAyatoConfig(flags *pflag.FlagSet) (*AyatoConfig, error) {
	// return loadConfig[AyatoConfig]("ayato_config.json")
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
