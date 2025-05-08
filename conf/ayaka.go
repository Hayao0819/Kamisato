package conf

type AyakaConfig struct {
	RepoDir string `koanf:"repodir"`
	DestDir string `koanf:"distdir"`
}

func LoadAyakaConfig() (*AyakaConfig, error) {
	return loadConfig[AyakaConfig](".ayakarc.toml", ".ayakarc.yaml", ".ayakarc.json")
}
