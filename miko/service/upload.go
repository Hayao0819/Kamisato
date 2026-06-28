package service

import (
	"os"

	blinky_clientlib "github.com/BrenekH/blinky/clientlib"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/pacman/gpg"
)

// signAndUpload signs each built package with the requested GPG key (when set)
// and uploads it with its detached signature to ayato via the blinky client.
// With Concurrency > 1 these share cfg.Build.GnupgHome, but gpg locks its own
// keyring, so concurrent signing is safe.
func signAndUpload(cfg *conf.MikoConfig, repo, gpgKey string, packages []string) error {
	client, err := blinky_clientlib.New(cfg.Ayato.URL, cfg.Ayato.Username, cfg.Ayato.Password)
	if err != nil {
		return utils.WrapErr(err, "failed to create blinky client")
	}

	for _, pkgPath := range packages {
		sigPath := ""
		if gpgKey != "" {
			if err := gpg.SignFile(gpgKey, cfg.Build.GnupgHome, pkgPath); err != nil {
				return utils.WrapErr(err, "failed to sign package: "+pkgPath)
			}
			sigPath = pkgPath + ".sig"
		}

		if err := uploadOne(client, repo, pkgPath, sigPath); err != nil {
			return err
		}
	}
	return nil
}

func uploadOne(client *blinky_clientlib.BlinkyClient, repo, pkgPath, sigPath string) error {
	pkgFile, err := os.Open(pkgPath)
	if err != nil {
		return utils.WrapErr(err, "failed to open package: "+pkgPath)
	}
	defer func() { _ = pkgFile.Close() }()

	// A nil signature reader tells the blinky client there is no signature.
	var sigFile *os.File
	if sigPath != "" {
		sigFile, err = os.Open(sigPath)
		if err != nil {
			return utils.WrapErr(err, "failed to open signature: "+sigPath)
		}
		defer func() { _ = sigFile.Close() }()
	}

	if sigFile == nil {
		if err := client.UploadPackage(repo, pkgPath, pkgFile, nil); err != nil {
			return utils.WrapErr(err, "failed to upload package: "+pkgPath)
		}
		return nil
	}
	if err := client.UploadPackage(repo, pkgPath, pkgFile, sigFile); err != nil {
		return utils.WrapErr(err, "failed to upload package: "+pkgPath)
	}
	return nil
}
