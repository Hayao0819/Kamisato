package source

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	configloader "github.com/Hayao0819/Kamisato/internal/config"
)

func (c *SrcConfig) Marshal() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

func LoadConfig(directory string) (*SrcConfig, error) {
	return configloader.LoadTyped[SrcConfig](
		[]string{directory},
		[]string{"repo.json"},
		nil,
		"REPO",
		(*SrcConfig).migrateLegacy,
	)
}

func (c *SrcConfig) migrateLegacy() {
	if c.LegacyServer != "" {
		slog.Warn("repo.json: top-level 'server' is deprecated; use 'url'")
		if c.URL == "" {
			c.URL = stripArchSuffix(c.LegacyServer)
		}
		c.LegacyServer = ""
	}
	if c.LegacyArchBuild != "" {
		slog.Warn("repo.json: archbuild is ignored for host safety; configure .ayakarc builder.devtools.archbuild")
		if c.Build.ArchBuild == "" {
			c.Build.ArchBuild = c.LegacyArchBuild
		}
		c.LegacyArchBuild = ""
	}
}

var archSuffixes = []string{"x86_64_v2", "x86_64_v3", "x86_64_v4", "x86_64", "pentium4", "aarch64", "armv7h", "i686", "i486", "any"}

func stripArchSuffix(server string) string {
	server = strings.TrimRight(server, "/")
	for _, arch := range archSuffixes {
		if base, ok := strings.CutSuffix(server, "/"+arch); ok {
			return base
		}
	}
	return server
}

func (c *SrcConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}
