package source

import (
	"io"
	"os"
	"os/exec"
	"path"
	"regexp"

	"github.com/Hayao0819/Kamisato/internal/errors"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/Hayao0819/Kamisato/pkg/safefile"
)

var pkgverRe = regexp.MustCompile(`(?m)^pkgver=['"]?([^\s'"#]+)['"]?[ \t]*(?:#[^\r\n]*)?\r?$`)

// NvBump rewrites name's pkgver to newVersion, resets pkgrel to 1, refreshes
// the checksums with updpkgsums (which downloads the new sources) and
// regenerates the .SRCINFO, returning the reloaded package.
func NvBump(src *repo.SourceRepo, name, newVersion string, stderr io.Writer) (*pkg.SourcePackage, error) {
	p := findPackage(src.Pkgs, name)
	if p == nil {
		return nil, errors.NewErr("package not found: " + name)
	}
	pkgbuild := path.Join(p.Dir(), "PKGBUILD")
	data, err := os.ReadFile(pkgbuild)
	if err != nil {
		return nil, errors.WrapErr(err, "failed to read PKGBUILD")
	}
	out, err := rewritePkgver(data, newVersion)
	if err != nil {
		return nil, errors.WrapErr(err, pkgbuild)
	}
	if err := safefile.WriteFile(pkgbuild, out, 0o644); err != nil { //nolint:gosec // PKGBUILD is world-readable source
		return nil, errors.WrapErr(err, "failed to write "+pkgbuild)
	}

	cmd := exec.Command("updpkgsums")
	cmd.Dir = p.Dir()
	cmd.Stdout = stderr
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return nil, errors.WrapErr(err, "updpkgsums failed in "+p.Dir())
	}

	if err := repo.GenerateSrcinfo(p.Dir(), stderr); err != nil {
		return nil, err
	}
	reloaded, err := pkg.OpenSourcePackage(p.Dir())
	if err != nil {
		return nil, errors.WrapErr(err, "failed to reload "+p.Base()+" after nvbump")
	}
	return reloaded, nil
}

// rewritePkgver splices newVersion into the pkgver value and resets pkgrel to
// 1, preserving quoting, comments and line endings like rewritePkgrel.
func rewritePkgver(data []byte, newVersion string) ([]byte, error) {
	m := pkgverRe.FindSubmatchIndex(data)
	if m == nil {
		return nil, errors.NewErr("pkgver not found")
	}
	var out []byte
	out = append(out, data[:m[2]]...)
	out = append(out, []byte(newVersion)...)
	out = append(out, data[m[3]:]...)

	r := pkgrelRe.FindSubmatchIndex(out)
	if r == nil {
		return nil, errors.NewErr("pkgrel not found")
	}
	var reset []byte
	reset = append(reset, out[:r[2]]...)
	reset = append(reset, '1')
	reset = append(reset, out[r[3]:]...)
	return reset, nil
}
