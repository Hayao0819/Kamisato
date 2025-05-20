package conf

type RepoConfig struct {
	Name       string `koanf:"name"`
	Maintainer string `koanf:"maintainer"`
	Server     string `koanf:"server"`
}

// func LoadRepoConfig(repodir string, config *RepoConfig) error {
// 	viper.SetConfigName("repo")
// 	viper.AddConfigPath(repodir)

// 	if err := viper.ReadInConfig(); err != nil {
// 		return err
// 	}

// 	if err := viper.Unmarshal(config); err != nil {
// 		return err
// 	}
// 	return nil
// }

func LoadRepoConfig(repodir string) (*RepoConfig, error) {
	// return loadConfigWithDir[RepoConfig]([]string{repodir}, []string{"repo.json"})

	return loadConfig[RepoConfig](
		[]string{repodir},
		[]string{"repo.json"},
		nil,
		"REPO",
	)
}
