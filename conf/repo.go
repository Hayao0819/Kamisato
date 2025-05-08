package conf

import (
	viper "github.com/spf13/viper"
)

type RepoConfig struct {
	Name       string `mapstructure:"name"`
	Maintainer string `mapstructure:"maintainer"`
}

func LoadRepoConfig(repodir string, config *RepoConfig) error {
	viper.SetConfigName("repo")
	viper.AddConfigPath(repodir)

	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	if err := viper.Unmarshal(config); err != nil {
		return err
	}
	return nil
}
