package source

import (
	"encoding/json"
	"os"
	"path/filepath"

	sourcerepo "github.com/Hayao0819/Kamisato/ayaka/source"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

type scaffoldConfig struct {
	Repos []scaffoldRepoEntry `json:"repos"`
	Debug bool                `json:"debug"`
}

type scaffoldRepoEntry struct {
	Dir     string `json:"dir"`
	DestDir string `json:"destdir"`
}

// Scaffolded reports where Scaffold placed the new repository's pieces.
type Scaffolded struct {
	TargetDir  string
	RepoDir    string
	DestDir    string
	ConfigPath string
}

// PrepareTargetDir ensures dir exists and is empty, returning its absolute
// path. A non-empty dir is refused so init never clobbers an existing repo.
func PrepareTargetDir(dir string) (string, error) {
	if contents, err := os.ReadDir(dir); err != nil {
		if !os.IsNotExist(err) {
			return "", errors.WrapErr(err, "failed to read target directory")
		}
		if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // G301: scaffolded repo world-readable by design
			return "", errors.WrapErr(err, "failed to create target directory")
		}
	} else if len(contents) > 0 {
		return "", &os.PathError{Op: "init", Path: dir, Err: os.ErrExist}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", errors.WrapErr(err, "failed to resolve target directory")
	}
	return abs, nil
}

// Scaffold writes a fresh .ayakarc.json plus one source repository (repo.json)
// and its output directory under targetDir.
func Scaffold(targetDir, repoName, maintainer, destDir string) (*Scaffolded, error) {
	if !filepath.IsLocal(repoName) || filepath.Base(repoName) != repoName || repoName == "." {
		return nil, errors.NewErr("repository name must be a single local directory name")
	}
	ayakarcPath := filepath.Join(targetDir, ".ayakarc.json")
	repoDir := filepath.Join(targetDir, repoName)

	relRepoDir, err := filepath.Rel(filepath.Dir(ayakarcPath), repoDir)
	if err != nil {
		return nil, errors.WrapErr(err, "failed to compute repository directory path")
	}
	relOutDir, err := filepath.Rel(filepath.Dir(ayakarcPath), destDir)
	if err != nil {
		return nil, errors.WrapErr(err, "failed to compute output directory path")
	}

	ayakarc := scaffoldConfig{
		Repos: []scaffoldRepoEntry{{
			Dir:     relRepoDir,
			DestDir: relOutDir,
		}},
		Debug: false,
	}
	ayakarcBytes, err := json.MarshalIndent(ayakarc, "", "  ")
	if err != nil {
		return nil, errors.WrapErr(err, "failed to marshal ayaka config")
	}
	if err := os.WriteFile(ayakarcPath, ayakarcBytes, 0o644); err != nil { //nolint:gosec // G306: scaffolded repo world-readable by design
		return nil, errors.WrapErr(err, "failed to write ayaka config")
	}

	if err := os.MkdirAll(repoDir, 0o755); err != nil { //nolint:gosec // G301: scaffolded repo world-readable by design
		return nil, errors.WrapErr(err, "failed to create repository directory")
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil { //nolint:gosec // G301: scaffolded repo world-readable by design
		return nil, errors.WrapErr(err, "failed to create output directory")
	}

	repoconf := sourcerepo.SrcConfig{
		Name:       repoName,
		Maintainer: maintainer,
	}
	repoconfBytes, err := repoconf.Marshal()
	if err != nil {
		return nil, errors.WrapErr(err, "failed to marshal repo config")
	}
	if err := os.WriteFile(filepath.Join(repoDir, "repo.json"), repoconfBytes, 0o644); err != nil { //nolint:gosec // G306: scaffolded repo world-readable by design
		return nil, errors.WrapErr(err, "failed to write repo config")
	}

	return &Scaffolded{
		TargetDir:  targetDir,
		RepoDir:    repoDir,
		DestDir:    destDir,
		ConfigPath: ayakarcPath,
	}, nil
}
