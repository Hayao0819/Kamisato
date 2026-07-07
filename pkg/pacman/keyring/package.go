package keyring

import (
	"archive/tar"
	"bytes"
	"fmt"
	"time"

	"github.com/klauspost/compress/zstd"

	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

// keyringDir is where pacman looks for keyring bundles.
const keyringDir = "usr/share/pacman/keyrings"

// PackageOpts describes the keyring package to build.
type PackageOpts struct {
	Files     *Files    // the three keyring artifacts (required)
	Version   string    // full pkgver including pkgrel, e.g. "20260707-1"
	Packager  string    // PKGINFO packager, e.g. "MyRepo <repo@example.com>"
	Desc      string    // pkgdesc; defaults to "<name> PGP keyring"
	License   []string  // optional license list
	Depends   []string  // optional depends (e.g. archlinux-keyring)
	BuildDate time.Time // stamped into PKGINFO and tar mtimes
}

// PkgName is the package name for a keyring identifier: "<name>-keyring".
func PkgName(name string) string { return name + "-keyring" }

// FileName is the conventional package filename: "<name>-keyring-<version>-any.pkg.tar.zst".
func FileName(name, version string) string {
	return fmt.Sprintf("%s-%s-any.pkg.tar.zst", PkgName(name), version)
}

// BuildPackage assembles the keyring .pkg.tar.zst (.PKGINFO first, then .INSTALL, then the three keyring files).
// The result is unsigned; callers sign it with a detached OpenPGP signature.
func BuildPackage(opts PackageOpts) ([]byte, error) {
	if opts.Files == nil {
		return nil, fmt.Errorf("keyring files are required")
	}
	if opts.Version == "" {
		return nil, fmt.Errorf("version is required")
	}
	f := opts.Files
	desc := opts.Desc
	if desc == "" {
		desc = f.Name + " PGP keyring"
	}
	buildDate := opts.BuildDate
	if buildDate.IsZero() {
		buildDate = time.Now()
	}

	info := raiou.NewPKGINFO()
	info.PkgName = PkgName(f.Name)
	info.PkgBase = PkgName(f.Name)
	info.PkgVer = opts.Version
	info.PkgDesc = desc
	info.Arch = "any"
	info.BuildDate = buildDate.Unix()
	info.Packager = opts.Packager
	info.Size = int64(len(f.GPG) + len(f.Trusted) + len(f.Revoked))
	info.License = opts.License
	info.Depend = opts.Depends
	info.PkgType = "pkg"

	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)

	writeFile := func(name string, mode int64, data []byte) error {
		if err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Typeflag: tar.TypeReg,
			Mode:     mode,
			Size:     int64(len(data)),
			ModTime:  buildDate,
		}); err != nil {
			return err
		}
		_, err := tw.Write(data)
		return err
	}
	writeDir := func(name string) error {
		return tw.WriteHeader(&tar.Header{
			Name:     name + "/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
			ModTime:  buildDate,
		})
	}

	// .PKGINFO must be the first member; the metadata dotfiles follow.
	if err := writeFile(".PKGINFO", 0o644, info.Bytes()); err != nil {
		return nil, err
	}
	if err := writeFile(".INSTALL", 0o644, []byte(installScript(f.Name))); err != nil {
		return nil, err
	}
	for _, dir := range []string{"usr", "usr/share", "usr/share/pacman", keyringDir} {
		if err := writeDir(dir); err != nil {
			return nil, err
		}
	}
	payload := []struct {
		name string
		data []byte
	}{
		{keyringDir + "/" + f.Name + ".gpg", f.GPG},
		{keyringDir + "/" + f.Name + "-trusted", f.Trusted},
		{keyringDir + "/" + f.Name + "-revoked", f.Revoked},
	}
	for _, p := range payload {
		if err := writeFile(p.name, 0o644, p.data); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}

	var out bytes.Buffer
	zw, err := zstd.NewWriter(&out)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(raw.Bytes()); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
