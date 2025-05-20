package conf

import "github.com/spf13/pflag"

type AyakaConfig struct {
	RepoDir string `koanf:"repodir"`
	DestDir string `koanf:"destdir"`
	Debug   bool   `koanf:"debug"`
}

func LoadAyakaConfig(flags *pflag.FlagSet) (*AyakaConfig, error) {
	return loadConfig[AyakaConfig](
		commonConfigDirs(),
		[]string{".ayakarc.json", ".ayakarc.toml", ".ayakarc.yaml"},
		flags,
		"AYAKA",
	)
}
