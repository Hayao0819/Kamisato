package conf

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/Hayao0819/Kamisato/internal/confloader"
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
	return confloader.LoadTyped[AyakaConfig](
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
	return confloader.LoadTyped[SrcRepoConfig](
		[]string{repodir},
		[]string{"repo.json"},
		nil,
		"REPO",
		nil,
	)
}

// Validate requires a name, the key every source repo is looked up by; an
// unnamed repo can never be resolved by the commands that address it by name.
func (c *SrcRepoConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}
