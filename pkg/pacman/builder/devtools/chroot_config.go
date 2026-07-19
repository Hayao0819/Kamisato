package devtools

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder/internal/buildenv"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder/internal/errutil"
)

var devtoolsDataDir = "/usr/share/devtools"

func renderChrootPacmanConf(repoName string, repos []builder.PacmanRepository) (string, error) {
	dir := filepath.Join(devtoolsDataDir, "pacman.conf.d")
	base := filepath.Join(dir, repoName+".conf")
	data, err := os.ReadFile(base) //nolint:gosec // path derived from a config repo name, not request input
	if errors.Is(err, os.ErrNotExist) {
		fallback := filepath.Join(dir, "extra.conf")
		data, err = os.ReadFile(fallback)
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("devtools pacman.conf not found (%s or %s); the 'devtools' package is required", base, fallback)
		}
	}
	if err != nil {
		return "", errutil.Wrap(err, "failed to read devtools pacman.conf")
	}
	stanzas, err := buildenv.PacmanRepoStanzas(repos)
	if err != nil {
		return "", err
	}
	return string(data) + stanzas, nil
}

// The base must remain complete because arch-nspawn copies it over /etc/makepkg.conf.
func renderChrootMakepkgConf(arch string, mk builder.MakepkgConfig) (string, error) {
	base := filepath.Join(devtoolsDataDir, "makepkg.conf.d", arch+".conf")
	data, err := os.ReadFile(base) //nolint:gosec // path derived from the target arch, not request input
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("devtools makepkg.conf not found (%s); the 'devtools' package is required", base)
	}
	if err != nil {
		return "", errutil.Wrap(err, "failed to read devtools makepkg.conf")
	}
	overrides, err := buildenv.MakepkgOverrideLines(mk)
	if err != nil {
		return "", err
	}
	// arch-nspawn copies this verbatim; separate the appended overrides from the
	// base's last line when the shipped file has no trailing newline.
	conf := string(data)
	if overrides != "" && !strings.HasSuffix(conf, "\n") {
		conf += "\n"
	}
	return conf + overrides, nil
}

func repoFromArchBuild(archBuild string) string {
	tag := strings.TrimSuffix(filepath.Base(archBuild), "-build")
	if i := strings.LastIndex(tag, "-"); i >= 0 {
		return tag[:i]
	}
	return tag
}

func mkarchrootArgs(arch, pacConf, mkConf, chrootRoot string) []string {
	return []string{
		"setarch", arch,
		"mkarchroot",
		"-C", pacConf,
		"-M", mkConf,
		chrootRoot, "base-devel",
	}
}

func makechrootpkgArgs(chrootDir string, installPkgs []string) []string {
	args := []string{"makechrootpkg", "-c", "-r", chrootDir}
	for _, pkg := range installPkgs {
		args = append(args, "-I", pkg)
	}
	return append(args, "--", "--syncdeps", "--noconfirm", "--log", "--holdver")
}
