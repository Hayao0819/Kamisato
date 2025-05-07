package conf

import (
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	RepoPath string
	DBPath   string
	Username string
	Password string
}

func LoadConfig() (Config, error) {
	var config Config
	configPath := os.Getenv("BLINKY_CONFIG")
	if configPath == "" {
		configPath = "/etc/blinky/config.toml"
	}

	_, err := toml.DecodeFile(configPath, &config)
	if os.IsNotExist(err) {
		// If the config file doesn't exist, use default values
		config.RepoPath = "/var/lib/blinky/repo"
		config.DBPath = "/var/lib/blinky/repo.db"
		config.Username = "admin"
		config.Password = "password"
		return config, nil
	}
	if err != nil {
		return Config{}, err
	}

	return config, nil
}
