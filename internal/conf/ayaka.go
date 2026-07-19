package conf

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	"github.com/spf13/pflag"
)

type RepoEntry struct {
	Dir     string `koanf:"dir" json:"dir"`
	DestDir string `koanf:"destdir" json:"destdir"`
}

type AyakaConfig struct {
	LegacyRepoDir string             `koanf:"repodir" json:"repodir"`
	LegacyDestDir string             `koanf:"destdir" json:"destdir"`
	Repos         []RepoEntry        `koanf:"repos" json:"repos"`
	Builder       builder.HostConfig `koanf:"builder" json:"builder,omitempty"`
	Debug         bool               `koanf:"debug" json:"debug"`
}

func (c *AyakaConfig) Marshal() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

func LoadAyakaConfigFrom(configFile string, flags *pflag.FlagSet) (*AyakaConfig, error) {
	var files []string
	if configFile != "" {
		absPath, err := filepath.Abs(configFile)
		if err != nil {
			return nil, err
		}
		files = []string{absPath}
	} else {
		files = configFileNames("", ".ayakarc")
	}
	return loadTypedWithSourceTransforms[AyakaConfig](
		commonConfigDirs(),
		files,
		flags,
		"AYAKA",
		(*AyakaConfig).migrateLegacy,
		validateBuilderSource,
	)
}

func LoadAyakaConfig(flags *pflag.FlagSet) (*AyakaConfig, error) {
	return LoadAyakaConfigFrom("", flags)
}

// migrateLegacy folds the deprecated top-level repodir/destdir into the repos
// list so the rest of the CLI only ever deals with the current shape.
func (c *AyakaConfig) migrateLegacy() {
	if c.LegacyRepoDir == "" && c.LegacyDestDir == "" {
		return
	}
	slog.Warn("Using legacy configuration fields 'repodir' or 'destdir' is deprecated. Please migrate to the new 'repos' field.")
	c.Repos = append(c.Repos, RepoEntry{Dir: c.LegacyRepoDir, DestDir: c.LegacyDestDir})
}

// Validate rejects a repos entry without a source directory, which would
// otherwise surface later as a confusing load failure.
func (c *AyakaConfig) Validate() error {
	if err := c.Builder.Validate(); err != nil {
		return fmt.Errorf("builder: %w", err)
	}
	for i, r := range c.Repos {
		if r.Dir == "" {
			return fmt.Errorf("repos[%d]: dir is required", i)
		}
	}
	return nil
}

type InstallPkgsConfig struct {
	Files []string `koanf:"files" json:"files"`
	Names []string `koanf:"names" json:"names"`
}

type SrcRepoConfig struct {
	Name        string                `koanf:"name" json:"name"`
	Maintainer  string                `koanf:"maintainer" json:"maintainer"`
	URL         string                `koanf:"url" json:"url,omitempty"`
	Build       builder.ProjectConfig `koanf:"build" json:"build,omitempty"`
	InstallPkgs InstallPkgsConfig     `koanf:"installpkgs" json:"installpkgs"`

	// ArchBuild is retained only to report that repository-owned commands are ignored.
	LegacyServer    string `koanf:"server" json:"server,omitempty"`
	LegacyArchBuild string `koanf:"archbuild" json:"archbuild,omitempty"`
}

func (c *SrcRepoConfig) Marshal() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

func LoadSrcRepoConfig(repodir string) (*SrcRepoConfig, error) {
	return LoadTyped[SrcRepoConfig](
		[]string{repodir},
		[]string{"repo.json"},
		nil,
		"REPO",
		(*SrcRepoConfig).migrateLegacy,
	)
}

func (c *SrcRepoConfig) migrateLegacy() {
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

// archSuffixes are the trailing arch segments stripArchSuffix trims so a legacy
// arch-ful server URL migrates to the arch-less url ayaka now appends --arch to.
var archSuffixes = []string{"x86_64_v2", "x86_64_v3", "x86_64_v4", "x86_64", "aarch64", "armv7h", "any"}

func stripArchSuffix(server string) string {
	s := strings.TrimRight(server, "/")
	for _, a := range archSuffixes {
		if before, ok := strings.CutSuffix(s, "/"+a); ok {
			return before
		}
	}
	return s
}

// Validate requires a name, the key every source repo is looked up by.
func (c *SrcRepoConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}
