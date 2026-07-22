package keyring

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"

	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
	"github.com/Hayao0819/Kamisato/pkg/safefile"
)

// BuildParams describe one keyring package build from a signing key.
type BuildParams struct {
	Name     string
	Version  string // empty applies DefaultVersion
	Packager string
	Desc     string
	Revoked  []string
	License  []string
	Depends  []string
	Sign     bool
}

// DefaultVersion is today's date with pkgrel 1, the convention every
// third-party keyring package follows; a non-empty version wins.
func DefaultVersion(version string) string {
	if version != "" {
		return version
	}
	return time.Now().Format("20060102") + "-1"
}

// MakePackage builds the keyring .pkg.tar.zst from k's public half into outDir
// and, when p.Sign is set, signs it. It returns the package path and the
// signature path (empty when unsigned).
func MakePackage(k *sign.SigningKey, p BuildParams, outDir string) (pkgPath, sigPath string, err error) {
	pub, err := k.PublicEntity()
	if err != nil {
		return "", "", fmt.Errorf("export public key: %w", err)
	}
	files, err := BuildFiles(p.Name, []*openpgp.Entity{pub}, []string{k.PrimaryFingerprint()}, p.Revoked)
	if err != nil {
		return "", "", fmt.Errorf("build keyring files: %w", err)
	}
	version := DefaultVersion(p.Version)
	data, err := BuildPackage(PackageOpts{
		Files:    files,
		Version:  version,
		Packager: p.Packager,
		Desc:     p.Desc,
		License:  p.License,
		Depends:  p.Depends,
	})
	if err != nil {
		return "", "", fmt.Errorf("build keyring package: %w", err)
	}

	pkgPath = filepath.Join(outDir, FileName(p.Name, version))
	// #nosec G306 -- this is a public package artifact intended for distribution.
	if err := safefile.WriteFile(pkgPath, data, 0o644); err != nil {
		return "", "", fmt.Errorf("write keyring package: %w", err)
	}
	if !p.Sign {
		return pkgPath, "", nil
	}
	sigPath, err = k.Sign(context.Background(), pkgPath)
	if err != nil {
		return "", "", fmt.Errorf("sign keyring package: %w", err)
	}
	return pkgPath, sigPath, nil
}
