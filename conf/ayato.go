package conf

import "path"

type AyatoConfig struct {
	RepoPath []string `koanf:"repopath"`
	Port     int      `koanf:"port"`
	DataPath string   `koanf:"datapath"`
	Username string   `koanf:"username"`
	Password string   `koanf:"password"`
}

func LoadAyatoConfig() (*AyatoConfig, error) {
	return loadConfig[AyatoConfig]("ayato_config.json")
}

func (c *AyatoConfig) DbPath() string {
	return path.Join(c.DataPath, "kv-db")
}
