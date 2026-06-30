package conf

import (
	"encoding/json"

	"github.com/spf13/pflag"
)

type RepoEntry struct {
	Dir     string `koanf:"dir" json:"dir"`
	DestDir string `koanf:"destdir" json:"destdir"`
}

type AyakaConfig struct {
	LegacyRepoDir string      `koanf:"repodir" json:"repodir"`
	LegacyDestDir string      `koanf:"destdir" json:"destdir"`
	Repos         []RepoEntry `koanf:"repos" json:"repos"`
	Debug         bool        `koanf:"debug" json:"debug"`
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

type SrcRepoConfig struct {
	Name        string `koanf:"name"`
	Maintainer  string `koanf:"maintainer"`
	ArchBuild   string `koanf:"archbuild"`
	Server      string `koanf:"server"`
	InstallPkgs struct {
		Files []string `koanf:"files"`
		Names []string `koanf:"names"`
	} `koanf:"installpkgs"`
}

func (c *SrcRepoConfig) Marshal() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

func LoadSrcRepoConfig(repodir string) (*SrcRepoConfig, error) {
	return loadConfig[SrcRepoConfig](
		[]string{repodir},
		[]string{"repo.json"},
		nil,
		"REPO",
	)
}
