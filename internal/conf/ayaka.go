package conf

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
)

type RepoEntry struct {
	Dir     string `koanf:"dir" json:"dir"`
	DestDir string `koanf:"destdir" json:"destdir"`
}

type AyakaConfig struct {
	LegacyRepoDir string      `koanf:"repodir" json:"repodir"`
	LegacyDestDir string      `koanf:"destdir" json:"destdir"`
	Repos         []RepoEntry `koanf:"repos" json:"repos"`
	Debug         bool        `koanf:"debug" json:"debug"`
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
	return LoadTyped[AyakaConfig](
		commonConfigDirs(),
		files,
		flags,
		"AYAKA",
		(*AyakaConfig).migrateLegacy,
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
	for i, r := range c.Repos {
		if r.Dir == "" {
			return fmt.Errorf("repos[%d]: dir is required", i)
		}
	}
	return nil
}

type BuildRepo struct {
	Name     string `koanf:"name" json:"name"`
	Server   string `koanf:"server" json:"server"`
	SigLevel string `koanf:"siglevel" json:"siglevel,omitempty"`
}

type MakepkgConfig struct {
	Packager     string   `koanf:"packager" json:"packager,omitempty"`
	Microarch    string   `koanf:"microarch" json:"microarch,omitempty"`
	CFlagsAppend string   `koanf:"cflags_append" json:"cflags_append,omitempty"`
	Options      []string `koanf:"options" json:"options,omitempty"`
}

type SrcBuildConfig struct {
	Repos     []BuildRepo   `koanf:"repos" json:"repos,omitempty"`
	Makepkg   MakepkgConfig `koanf:"makepkg" json:"makepkg,omitempty"`
	Arches    []string      `koanf:"arches" json:"arches,omitempty"`
	Image     string        `koanf:"image" json:"image,omitempty"`
	ArchBuild string        `koanf:"archbuild" json:"archbuild,omitempty"`
	Timeout   string        `koanf:"timeout" json:"timeout,omitempty"`
}

type InstallPkgsConfig struct {
	Files []string `koanf:"files" json:"files"`
	Names []string `koanf:"names" json:"names"`
}

type SrcRepoConfig struct {
	Name        string            `koanf:"name" json:"name"`
	Maintainer  string            `koanf:"maintainer" json:"maintainer"`
	URL         string            `koanf:"url" json:"url,omitempty"`
	Build       SrcBuildConfig    `koanf:"build" json:"build,omitempty"`
	InstallPkgs InstallPkgsConfig `koanf:"installpkgs" json:"installpkgs"`

	// Deprecated top-level aliases, folded into URL / Build.ArchBuild by migrateLegacy.
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

// migrateLegacy folds the deprecated top-level server/archbuild into the current
// url / build.archbuild shape so the rest of the CLI only sees the new fields.
func (c *SrcRepoConfig) migrateLegacy() {
	if c.LegacyServer != "" {
		slog.Warn("repo.json: top-level 'server' is deprecated; use 'url'")
		if c.URL == "" {
			c.URL = stripArchSuffix(c.LegacyServer)
		}
		c.LegacyServer = ""
	}
	if c.LegacyArchBuild != "" {
		slog.Warn("repo.json: top-level 'archbuild' is deprecated; use 'build.archbuild'")
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
		if strings.HasSuffix(s, "/"+a) {
			return strings.TrimSuffix(s, "/"+a)
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
