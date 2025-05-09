package conf

type AyatoConfig struct {
	RepoPath []string
	Port     int
	DBPath   string
	Username string
	Password string
}

func LoadAyatoConfig() (*AyatoConfig, error) {
	return loadConfig[AyatoConfig]("ayatorc.json", "ayatorc.yaml", "ayatorc.toml")
}
