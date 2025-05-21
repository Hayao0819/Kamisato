package conf

import (
	"fmt"
	"path"

	"github.com/spf13/pflag"
)

type AyatoConfig struct {
	RepoPath []string `koanf:"repopath"`
	Port     int      `koanf:"port"`
	DataPath string   `koanf:"datapath"`
	Username string   `koanf:"username"`
	Password string   `koanf:"password"`
	MaxSize  int      `koanf:"maxsize"`
	Database DbConfig `koanf:"dbconfig"`
}

type DbConfig struct {
	Driver   string `koanf:"driver"`
	Server   string `koanf:"server"`
	User     string `koanf:"user"`
	Password string `koanf:"password"`
	Database string `koanf:"database"`
}

func (d *DbConfig) DSN() string {
	return fmt.Sprintf("%s:%s@%s/%s?charset=%s&parseTime=%s", d.User, d.Password, d.Server, d.Database, "utf8", "true")
}

func (a *AyatoConfig) Db() (string, string) {
	return a.Database.Driver, a.Database.DSN()
}

func LoadAyatoConfig(flags *pflag.FlagSet) (*AyatoConfig, error) {
	// return loadConfig[AyatoConfig]("ayato_config.json")
	return loadConfig[AyatoConfig](
		commonConfigDirs(),
		[]string{"ayato_config.json", "ayato_config.toml", "ayato_config.yaml"},
		flags,
		"AYATO",
	)

}

func (c *AyatoConfig) DbPath() string {
	return path.Join(c.DataPath, "kv-db")
}
