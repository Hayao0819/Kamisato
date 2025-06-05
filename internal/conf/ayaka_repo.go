package conf

type SrcRepoConfig struct {
	Name        string `koanf:"name"`
	Maintainer  string `koanf:"maintainer"`
	Server      string `koanf:"server"`
	InstallPkgs struct {
		Files []string `koanf:"files"`
		Names []string `koanf:"names"`
	} `koanf:"installpkgs"`
	// AurPkgs    []string `koanf:"aurpkgs"`
}

func LoadSrcRepoConfig(repodir string) (*SrcRepoConfig, error) {
	return loadConfig[SrcRepoConfig](
		[]string{repodir},
		[]string{"repo.json"},
		nil,
		"REPO",
	)
}
