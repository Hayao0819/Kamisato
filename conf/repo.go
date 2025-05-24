package conf

type RepoConfig struct {
	Name        string `koanf:"name"`
	Maintainer  string `koanf:"maintainer"`
	Server      string `koanf:"server"`
	InstallPkgs struct {
		Files []string `koanf:"files"`
		// Names []string `koanf:"names"`
	} `koanf:"installpkgs"`
	// AurPkgs    []string `koanf:"aurpkgs"`
}

func LoadRepoConfig(repodir string) (*RepoConfig, error) {
	return loadConfig[RepoConfig](
		[]string{repodir},
		[]string{"repo.json"},
		nil,
		"REPO",
	)
}
