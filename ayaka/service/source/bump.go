package source

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/samber/lo"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/gitcmd"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/Hayao0819/Kamisato/pkg/safefile"
)

var pkgrelRe = regexp.MustCompile(`(?m)^pkgrel=['"]?([0-9]+(?:\.[0-9]+)?)['"]?[ \t]*(?:#[^\r\n]*)?\r?$`)

// BumpPkgrel raises the pkgrel of each named source package in its PKGBUILD and
// regenerates the .SRCINFO, returning the edited packages. by is "0.1" (rebuild
// suffix: 1 -> 1.1 -> 1.2) or "1" (next integer: 1.2 -> 2).
func BumpPkgrel(src *repo.SourceRepo, names []string, by string, stderr io.Writer) ([]*pkg.SourcePackage, error) {
	var bumped []*pkg.SourcePackage
	for _, name := range names {
		p := findPackage(src.Pkgs, name)
		if p == nil {
			return nil, errors.NewErr("package not found: " + name)
		}
		pkgbuild := path.Join(p.Dir(), "PKGBUILD")
		data, err := os.ReadFile(pkgbuild)
		if err != nil {
			return nil, errors.WrapErr(err, "failed to read PKGBUILD")
		}
		out, err := rewritePkgrel(data, by)
		if err != nil {
			return nil, errors.WrapErr(err, pkgbuild)
		}
		if err := safefile.WriteFile(pkgbuild, out, 0o644); err != nil { //nolint:gosec // PKGBUILD is world-readable source
			return nil, errors.WrapErr(err, "failed to write "+pkgbuild)
		}
		if err := repo.GenerateSrcinfo(p.Dir(), stderr); err != nil {
			return nil, err
		}
		reloaded, err := pkg.OpenSourcePackage(p.Dir())
		if err != nil {
			return nil, errors.WrapErr(err, "failed to reload "+p.Base()+" after bump")
		}
		bumped = append(bumped, reloaded)
	}
	return bumped, nil
}

// CommitBump stages each bumped package's PKGBUILD/.SRCINFO and commits them in
// the git repo containing srcDir, returning the commit hash.
func CommitBump(srcDir string, bumped []*pkg.SourcePackage, message string) (string, error) {
	root, err := gitcmd.RepoRoot(srcDir)
	if err != nil {
		return "", err
	}
	var paths []string
	for _, p := range bumped {
		for _, f := range []string{"PKGBUILD", ".SRCINFO"} {
			rel, err := filepath.Rel(root, filepath.Join(p.Dir(), f))
			if err != nil {
				return "", errors.WrapErr(err, "failed to resolve path for commit")
			}
			paths = append(paths, filepath.ToSlash(rel))
		}
	}
	return gitcmd.CommitPaths(root, paths, message)
}

func findPackage(pkgs []*pkg.SourcePackage, name string) *pkg.SourcePackage {
	p, _ := lo.Find(pkgs, func(p *pkg.SourcePackage) bool {
		return name == p.Base() || lo.Contains(p.Names(), name)
	})
	return p
}

// rewritePkgrel replaces the pkgrel value in a PKGBUILD with the next value per
// by. Only the value span is spliced, so quoting, comments and line endings
// around it survive.
func rewritePkgrel(data []byte, by string) ([]byte, error) {
	m := pkgrelRe.FindSubmatchIndex(data)
	if m == nil {
		return nil, errors.NewErr("pkgrel not found")
	}
	next, err := nextPkgrel(string(data[m[2]:m[3]]), by)
	if err != nil {
		return nil, err
	}
	var out []byte
	out = append(out, data[:m[2]]...)
	out = append(out, []byte(next)...)
	out = append(out, data[m[3]:]...)
	return out, nil
}

// nextPkgrel steps rel by "0.1" (append or raise the rebuild suffix) or "1"
// (next integer, dropping any suffix).
func nextPkgrel(cur, by string) (string, error) {
	intPart, frac, hasFrac := strings.Cut(cur, ".")
	n, err := strconv.Atoi(intPart)
	if err != nil {
		return "", errors.NewErr("invalid pkgrel: " + cur)
	}
	switch by {
	case "1":
		return strconv.Itoa(n + 1), nil
	case "0.1":
		if !hasFrac {
			return intPart + ".1", nil
		}
		f, err := strconv.Atoi(frac)
		if err != nil {
			return "", errors.NewErr("invalid pkgrel: " + cur)
		}
		return intPart + "." + strconv.Itoa(f+1), nil
	}
	return "", errors.NewErr("invalid bump step: " + by + " (0.1 or 1)")
}
