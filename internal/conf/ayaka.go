package conf

import (
	"encoding/json"

	"github.com/spf13/pflag"
)

type AyakaConfig struct {
	LegacyRepoDir string `koanf:"repodir" json:"repodir"`
	LegacyDestDir string `koanf:"destdir" json:"destdir"`
	Repos         []struct {
		Dir     string `koanf:"dir" json:"dir"`
		DestDir string `koanf:"destdir" json:"destdir"`
	} `koanf:"repos" json:"repos"`
	Debug bool `koanf:"debug" json:"debug"`
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
