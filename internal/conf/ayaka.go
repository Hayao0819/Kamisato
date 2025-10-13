package conf

import (
	"encoding/json"

	"github.com/spf13/pflag"
)

type AyakaConfig struct {
	RepoDir string `koanf:"repodir" json:"repodir"`
	DestDir string `koanf:"destdir" json:"destdir"`
	Debug   bool   `koanf:"debug" json:"debug"`
}

func (c *AyakaConfig) Marshal() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

func LoadAyakaConfig(flags *pflag.FlagSet) (*AyakaConfig, error) {
	return loadConfig[AyakaConfig](
		commonConfigDirs(),
		[]string{".ayakarc.json", ".ayakarc.toml", ".ayakarc.yaml"},
		flags,
		"AYAKA",
	)
}
