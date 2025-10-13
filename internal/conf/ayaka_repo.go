package conf

import "encoding/json"

type SrcRepoConfig struct {
	Name        string `koanf:"name"`
	Maintainer  string `koanf:"maintainer"`
	ArchBuild   string `koanf:"archbuild"`
	Server      string `koanf:"server"`
	InstallPkgs struct {
		Files []string `koanf:"files"`
		Names []string `koanf:"names"`
	} `koanf:"installpkgs"`
	// AurPkgs    []string `koanf:"aurpkgs"`
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
