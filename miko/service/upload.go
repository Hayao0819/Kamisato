package service

import (
	"context"
	"os"

	blinky_clientlib "github.com/BrenekH/blinky/clientlib"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// signAndUpload signs each built package with the worker host key (when a signer
// is configured) and uploads it with its detached signature to ayato.
func (s *Service) signAndUpload(ctx context.Context, repo string, packages []string) error {
	client, err := blinky_clientlib.New(s.cfg.Ayato.URL, s.cfg.Ayato.Username, s.cfg.Ayato.Password)
	if err != nil {
		return utils.WrapErr(err, "failed to create blinky client")
	}

	for _, pkgPath := range packages {
		sigPath := ""
		if s.signer != nil {
			sigPath, err = s.signer.Sign(ctx, pkgPath)
			if err != nil {
				return utils.WrapErr(err, "failed to sign package: "+pkgPath)
			}
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
