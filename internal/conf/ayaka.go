package conf

import (
	"encoding/json"
	"fmt"
	"log/slog"

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

func LoadAyakaConfig(flags *pflag.FlagSet) (*AyakaConfig, error) {
	cfg, err := loadConfig[AyakaConfig](
		commonConfigDirs(),
		[]string{".ayakarc.json", ".ayakarc.toml", ".ayakarc.yaml"},
		flags,
		"AYAKA",
	)
	if err != nil {
		return nil, err
	}
	cfg.migrateLegacy()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
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

type SrcRepoConfig struct {
	Name        string `koanf:"name"`
	Maintainer  string `koanf:"maintainer"`
	ArchBuild   string `koanf:"archbuild"`
	Server      string `koanf:"server"`
	InstallPkgs struct {
		Files []string `koanf:"files"`
		Names []string `koanf:"names"`
	} `koanf:"installpkgs"`
}

func (c *SrcRepoConfig) Marshal() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

func LoadSrcRepoConfig(repodir string) (*SrcRepoConfig, error) {
	return loadConfig[SrcRepoConfig](
		[]string{repodir},
		[]string{"repo.json"},
		nil,
		"REPO",
	)
}
