package conf

import (
	"fmt"

	"github.com/spf13/pflag"
)

// Thoma build modes. Ayato (the default) delegates through an ayato server and
// downloads the published, host-signed package; Direct talks to a miko builder
// itself and pulls the unsigned artifact straight from the job.
const (
	ThomaModeAyato  = "ayato"
	ThomaModeDirect = "direct"
)

// ThomaConfig configures the makepkg shim. Values come from THOMA_* env vars, a
// .thomarc.{json,toml,yaml}, or flags, in the usual koanf precedence.
type ThomaConfig struct {
	Repo    string `koanf:"repo"`
	Server  string `koanf:"server"`
	Arch    string `koanf:"arch"`
	ApiKey  string `koanf:"api_key"`
	Mode    string `koanf:"mode"`
	Makepkg string `koanf:"makepkg"`
	Timeout int    `koanf:"timeout"`
}

func LoadThomaConfig(flags *pflag.FlagSet) (*ThomaConfig, error) {
	loadDotEnv()
	cfg, err := loadConfig[ThomaConfig](
		commonConfigDirs(),
		[]string{".thomarc.json", ".thomarc.toml", ".thomarc.yaml"},
		flags,
		"THOMA",
	)
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Direct reports whether the config selects the direct-to-miko build mode.
func (c *ThomaConfig) Direct() bool { return c.Mode == ThomaModeDirect }

// Validate rejects a config that cannot produce a build: Repo is always needed,
// an unknown Mode is a typo rather than a silent fallback, and direct mode needs
// the miko api key because it authenticates to miko without ayato's CLI token.
func (c *ThomaConfig) Validate() error {
	if c.Repo == "" {
		return fmt.Errorf("repo: required (set THOMA_REPO or repo in .thomarc)")
	}
	switch c.Mode {
	case "", ThomaModeAyato, ThomaModeDirect:
	default:
		return fmt.Errorf("mode: unknown value %q (want %q or %q)", c.Mode, ThomaModeAyato, ThomaModeDirect)
	}
	if c.Direct() && c.ApiKey == "" {
		return fmt.Errorf("api_key: required in direct mode (set THOMA_API_KEY)")
	}
	return nil
}
