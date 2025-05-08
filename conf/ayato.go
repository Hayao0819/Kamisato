package conf

type AyatoConfig struct {
	RepoPath string
	DBPath   string
	Username string
	Password string
}

func LoadAyatoConfig() (*AyatoConfig, error) {
	return loadConfig[AyatoConfig]("ayatorc.json", "ayatorc.yaml", "ayatorc.toml")
}
