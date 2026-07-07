package builder

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// devtoolsDataDir is the devtools config dir; a var so tests can substitute a fixture dir.
var devtoolsDataDir = "/usr/share/devtools"

// renderChrootPacmanConf returns the devtools pacman.conf for repoName (falling back to extra.conf) with
// per-build repo stanzas appended for -C. The base is kept complete: arch-nspawn copies it verbatim.
func renderChrootPacmanConf(repoName string, repos []RepoSpec) (string, error) {
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
		return "", wrapErr(err, "failed to read devtools pacman.conf")
	}
	return string(data) + pacmanRepoStanzas(repos), nil
}

// renderChrootMakepkgConf returns the devtools makepkg.conf for arch with override lines appended.
// No `source` directive is added: arch-nspawn copies this verbatim into /etc/makepkg.conf, so a source would self-recurse.
func renderChrootMakepkgConf(arch string, mk MakepkgSettings) (string, error) {
	base := filepath.Join(devtoolsDataDir, "makepkg.conf.d", arch+".conf")
	data, err := os.ReadFile(base) //nolint:gosec // path derived from the target arch, not request input
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("devtools makepkg.conf not found (%s); the 'devtools' package is required", base)
	}
	if err != nil {
		return "", wrapErr(err, "failed to read devtools makepkg.conf")
	}
	overrides, err := makepkgOverrideLines(mk)
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

// repoFromArchBuild strips "-build" then takes everything before the last '-', e.g. "alterlinux-x86_64-build" -> "alterlinux".
func repoFromArchBuild(archBuild string) string {
	tag := strings.TrimSuffix(filepath.Base(archBuild), "-build")
	if i := strings.LastIndex(tag, "-"); i >= 0 {
		return tag[:i]
	}
	return tag
}

// mkarchrootArgs assembles 'setarch <arch> mkarchroot -C <pac> -M <mk> <chrootRoot> base-devel'.
func mkarchrootArgs(arch, pacConf, mkConf, chrootRoot string) []string {
	return []string{
		"setarch", arch,
		"mkarchroot",
		"-C", pacConf,
		"-M", mkConf,
		chrootRoot, "base-devel",
	}
}

// makechrootpkgArgs assembles 'makechrootpkg -c -r <chrootDir> [-I pkg]... -- --syncdeps --noconfirm --log --holdver'.
func makechrootpkgArgs(chrootDir string, installPkgs []string) []string {
	args := []string{"makechrootpkg", "-c", "-r", chrootDir}
	for _, pkg := range installPkgs {
		args = append(args, "-I", pkg)
	}
	return append(args, "--", "--syncdeps", "--noconfirm", "--log", "--holdver")
}
