package builder

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// devtoolsDataDir is where devtools ships its pacman/makepkg config pairs. A var
// so tests can point it at a fixture dir.
var devtoolsDataDir = "/usr/share/devtools"

// renderChrootPacmanConf returns a complete pacman.conf for the clean chroot: the
// devtools base for repoName (falling back to extra.conf when that repo has no
// base) with the per-build repo stanzas appended, so build.repos reach mkarchroot
// via -C. The base must stay complete because arch-nspawn copies it into the
// chroot verbatim.
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

// renderChrootMakepkgConf returns a complete makepkg.conf for the clean chroot: the
// devtools base for arch with the per-build override lines appended inline (the
// base is complete, so the overrides come last for gcc/makepkg to honour). It adds
// no `source` line: arch-nspawn copies this file into the chroot's
// /etc/makepkg.conf verbatim, so a source of that path would self-recurse.
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

// repoFromArchBuild derives the repo name from a devtools wrapper the way archbuild
// does: strip the "-build" suffix, then take everything before the last '-' (the
// arch), e.g. "alterlinux-x86_64-build" -> "alterlinux".
func repoFromArchBuild(archBuild string) string {
	tag := strings.TrimSuffix(filepath.Base(archBuild), "-build")
	if i := strings.LastIndex(tag, "-"); i >= 0 {
		return tag[:i]
	}
	return tag
}

// mkarchrootArgs assembles the argv that creates the clean chroot with the
// generated pacman.conf (-C) and makepkg.conf (-M):
// setarch <arch> mkarchroot -C <pac> -M <mk> <chrootRoot> base-devel.
func mkarchrootArgs(arch, pacConf, mkConf, chrootRoot string) []string {
	return []string{
		"setarch", arch,
		"mkarchroot",
		"-C", pacConf,
		"-M", mkConf,
		chrootRoot, "base-devel",
	}
}

// makechrootpkgArgs assembles the full build argv, including the trailing makepkg
// args after the `--`:
// makechrootpkg -c -r <chrootDir> [-I pkg]... -- --syncdeps --noconfirm --log --holdver.
func makechrootpkgArgs(chrootDir string, installPkgs []string) []string {
	args := []string{"makechrootpkg", "-c", "-r", chrootDir}
	for _, pkg := range installPkgs {
		args = append(args, "-I", pkg)
	}
	return append(args, "--", "--syncdeps", "--noconfirm", "--log", "--holdver")
}
