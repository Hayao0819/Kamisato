package conf

import "path"

type AyatoConfig struct {
	RepoPath []string
	Port     int
	// DBPath   string
	DataPath string
	Username string
	Password string
}

func LoadAyatoConfig() (*AyatoConfig, error) {
	return loadConfig[AyatoConfig]("ayatorc.json", "ayatorc.yaml", "ayatorc.toml")
}

func (c *AyatoConfig) DbPath() string {
	return path.Join(c.DataPath, "kv-db")
}
