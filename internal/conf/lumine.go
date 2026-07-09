package conf

import (
	"fmt"

	"github.com/spf13/pflag"
)

// LumineConfig configures the lumine web frontend/BFF for ayato. AyatoURL and
// AuthMode decide whether lumine reverse-proxies ayato same-origin (cookie mode)
// or hands the SPA ayato's cross-origin URL to call with a bearer token.
type LumineConfig struct {
	Debug bool   `koanf:"debug"`
	Addr  string `koanf:"addr"` // listen address; default ":8080"
	// AyatoURL is the ayato base URL. In cookie mode /api and /repo are proxied
	// there; in bearer mode the SPA calls it directly. Also set via LUMINE_AYATO_URL.
	AyatoURL string `koanf:"ayato_url"`
	AuthMode string `koanf:"auth_mode"` // "cookie" (default) or "bearer"
	// Title and Description override the SPA's landing heading and subtitle. Empty
	// keeps the SPA's built-in default. Set via LUMINE_TITLE / LUMINE_DESCRIPTION.
	Title       string `koanf:"title"`
	Description string `koanf:"description"`
}

func LoadLumineConfig(flags *pflag.FlagSet, configFile string) (*LumineConfig, error) {
	loadDotEnv()
	return LoadTyped[LumineConfig](
		commonConfigDirs(),
		configFileNames(configFile, "lumine_config"),
		flags,
		"LUMINE",
		func(c *LumineConfig) {
			if c.Addr == "" {
				c.Addr = ":8080"
			}
			if c.AuthMode == "" {
				c.AuthMode = "cookie"
			}
		},
	)
}

// Validate rejects an unknown auth mode and requires ayato_url in bearer mode,
// where there is no same-origin proxy for the SPA to fall back on.
func (c *LumineConfig) Validate() error {
	switch c.AuthMode {
	case "cookie":
	case "bearer":
		if c.AyatoURL == "" {
			return fmt.Errorf("auth_mode \"bearer\" requires ayato_url (the cross-origin ayato URL the SPA calls directly)")
		}
	default:
		return fmt.Errorf("auth_mode must be \"cookie\" or \"bearer\", got %q", c.AuthMode)
	}
	return nil
}
