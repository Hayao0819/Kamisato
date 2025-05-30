package conf

import (
	"log/slog"
	"path"

	"github.com/spf13/pflag"
)

type AyatoConfig struct {
	Debug     bool        `koanf:"debug"`
	RepoNames []string    `koanf:"names"`
	Port      int         `koanf:"port"`
	MaxSize   int         `koanf:"maxsize"`
	Auth      AuthConfig  `koanf:"auth"`
	Store     StoreConfig `koanf:"store"`
}

type AuthConfig struct {
	Username string `koanf:"username"`
	Password string `koanf:"password"`
}

type StoreConfig struct {
	DBType       string     `koanf:"dbtype"` // "external", "cfkv" or "badgerdb"
	SQL          SqlConfig  `koanf:"sql"`
	StorageType  string     `koanf:"storagetype"` // "localfs" or "s3"
	RepoDir      string     `koanf:"repodir"`
	AWSS3        S3Config   `koanf:"awss3"`
	BadgerDB     string     `koanf:"badgerdb"`
	CloudflareKV CFKVConfig `koanf:"cfkv"`
}

func LoadAyatoConfig(flags *pflag.FlagSet) (*AyatoConfig, error) {
	// return loadConfig[AyatoConfig]("ayato_config.json")

	if err := LoadEnv(); err != nil {
		slog.Error("Failed to load env", "error", err)
	}

	return loadConfig[AyatoConfig](
		commonConfigDirs(),
		[]string{"ayato_config.json", "ayato_config.toml", "ayato_config.yaml"},
		flags,
		"AYATO",
	)

}

func (c *AyatoConfig) DbPath() string {
	return path.Join(c.Store.BadgerDB, "kv-db")
}
