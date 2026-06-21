package conf

import (
	"log/slog"
	"path"

	"github.com/spf13/pflag"
)

type AyatoConfig struct {
	Debug       bool            `koanf:"debug"`
	RequireSign bool            `koanf:"require_sign"`
	Port        int             `koanf:"port"`
	MaxSize     int             `koanf:"maxsize"`
	Repos       []BinRepoConfig `koanf:"repos"`
	Auth        AuthConfig      `koanf:"auth"`
	Store       StoreConfig     `koanf:"store"`
	Build       BuildConfig     `koanf:"build"`
	Miko        MikoUpstream    `koanf:"miko"`
}

// MikoUpstream is the internal build server ayato proxies build/job requests to.
type MikoUpstream struct {
	URL    string `koanf:"url"`     // internal base URL, e.g. http://miko:8081
	APIKey string `koanf:"api_key"` // shared secret sent to miko on every proxied call
}

type BuildConfig struct {
	Image     string `koanf:"image"`      // Docker image (default: "archlinux:latest")
	Timeout   int    `koanf:"timeout"`    // Build timeout in minutes (default: 30)
	GnupgHome string `koanf:"gnupg_home"` // GPG home directory for signing
}

type AuthConfig struct {
	Username string `koanf:"username"`
	Password string `koanf:"password"`
}

type BinRepoConfig struct {
	Name   string   `koanf:"name"`
	Arches []string `koanf:"arches"`
}

func LoadAyatoConfig(flags *pflag.FlagSet, configFile string) (*AyatoConfig, error) {
	// return loadConfig[AyatoConfig]("ayato_config.json")

	if err := LoadEnv(); err != nil {
		slog.Error("Failed to load env", "error", err)
	}

	dirs := commonConfigDirs()
	files := []string{}
	if configFile != "" {
		files = append(files, configFile)
	} else {
		files = []string{"ayato_config.json", "ayato_config.toml", "ayato_config.yaml"}
	}

	return loadConfig[AyatoConfig](
		dirs,
		files,
		flags,
		"AYATO",
	)
}

func (c *AyatoConfig) DbPath() string {
	return path.Join(c.Store.BadgerDB, "kv-db")
}

func (c *AyatoConfig) RepoNames() []string {
	repoNames := make([]string, len(c.Repos))
	for i, repo := range c.Repos {
		repoNames[i] = repo.Name
	}
	return repoNames
}

func (c *AyatoConfig) Repo(name string) *BinRepoConfig {
	for _, repo := range c.Repos {
		if repo.Name == name {
			return &repo
		}
	}
	return nil
}
