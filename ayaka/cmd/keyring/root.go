// Package keyringcmd implements `ayaka keyring`: it turns the public half of the
// repository signing key (managed by `ayaka key`) into a distributable pacman
// keyring package and publishes it. The private key never leaves the local
// machine; only public key material is packaged.
package keyringcmd

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/pkg/pacman/keyring"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

// Cmd builds the `ayaka keyring` command group.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keyring",
		Short: "Build and publish the repository keyring package",
		Long:  "Package the signing key's public half as a pacman keyring (the <name>.gpg + -trusted + -revoked files plus a populate hook) and publish it to the repository so users can trust it.",
	}
	shared.AddKeyFlags(cmd)
	cmd.AddCommand(
		buildCmd(),
		publishCmd(),
		bootstrapCmd(),
	)
	return cmd
}

// buildParams are the flags shared by `keyring build` and `keyring publish`.
type buildParams struct {
	name     string
	version  string
	packager string
	desc     string
	revoked  []string
	license  []string
	depends  []string
	sign     bool
}

func addBuildFlags(cmd *cobra.Command, p *buildParams) {
	cmd.Flags().StringVar(&p.name, "name", "", "Keyring identifier (the <name>.gpg stem and pacman-key --populate argument) (required)")
	cmd.Flags().StringVar(&p.version, "version", "", "Package version (default: today's date, e.g. 20260707-1)")
	cmd.Flags().StringVar(&p.packager, "packager", "", "PKGINFO packager field")
	cmd.Flags().StringVar(&p.desc, "desc", "", "Package description (default: '<name> PGP keyring')")
	cmd.Flags().StringSliceVar(&p.revoked, "revoked", nil, "Extra primary fingerprints to list as revoked (repeatable)")
	cmd.Flags().StringSliceVar(&p.license, "license", nil, "License(s) for PKGINFO (repeatable)")
	cmd.Flags().StringSliceVar(&p.depends, "depends", nil, "Package dependencies (repeatable)")
	cmd.Flags().BoolVar(&p.sign, "sign", true, "Sign the built package with the signing key")
	_ = cmd.MarkFlagRequired("name")
}

// defaultVersion is today's date with pkgrel 1, the convention every third-party
// keyring package follows.
func defaultVersion(p buildParams) string {
	if p.version != "" {
		return p.version
	}
	return time.Now().Format("20060102") + "-1"
}

// makePackage builds the keyring .pkg.tar.zst into outDir and, when p.sign is set,
// signs it. It returns the package path and the signature path (empty when
// unsigned).
func makePackage(k *sign.SigningKey, p buildParams, outDir string) (pkgPath, sigPath string, err error) {
	pub, err := k.PublicEntity()
	if err != nil {
		return "", "", errwrap.WrapErr(err, "export public key")
	}
	files, err := keyring.BuildFiles(p.name, []*openpgp.Entity{pub}, []string{k.PrimaryFingerprint()}, p.revoked)
	if err != nil {
		return "", "", errwrap.WrapErr(err, "build keyring files")
	}
	version := defaultVersion(p)
	data, err := keyring.BuildPackage(keyring.PackageOpts{
		Files:    files,
		Version:  version,
		Packager: p.packager,
		Desc:     p.desc,
		License:  p.license,
		Depends:  p.depends,
	})
	if err != nil {
		return "", "", errwrap.WrapErr(err, "build keyring package")
	}

	pkgPath = filepath.Join(outDir, keyring.FileName(p.name, version))
	if err := os.WriteFile(pkgPath, data, 0o644); err != nil {
		return "", "", errwrap.WrapErr(err, "write keyring package")
	}
	if !p.sign {
		return pkgPath, "", nil
	}
	sigPath, err = k.Sign(context.Background(), pkgPath)
	if err != nil {
		return "", "", errwrap.WrapErr(err, "sign keyring package")
	}
	return pkgPath, sigPath, nil
}
