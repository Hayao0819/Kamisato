package service

import (
	"context"
	"fmt"
	"os"

	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/limits"
)

// Uploader publishes a built package (with its optional detached signature) to a
// repo on ayato. The service depends on this seam rather than the blinky client
// directly so a fake can stand in for tests; blinkyUploader is the production one.
type Uploader interface {
	Upload(repo, pkgPath, sigPath string) error
}

var _ Uploader = (*blinkyUploader)(nil)

type blinkyUploader struct {
	info *blinkyutils.ServerInfo
}

// NewBlinkyUploader returns the ayato-backed Uploader injected into the service
// in production.
func NewBlinkyUploader(url, username, password string) Uploader {
	return &blinkyUploader{info: &blinkyutils.ServerInfo{URL: url, Username: username, Password: password}}
}

func (u *blinkyUploader) Upload(repo, pkgPath, sigPath string) error {
	client, err := u.info.Client()
	if err != nil {
		return errors.WrapErr(err, "failed to create blinky client")
	}
	if err := blinkyutils.Upload(client, repo, pkgPath, sigPath); err != nil {
		return errors.WrapErr(err, "failed to upload package: "+pkgPath)
	}
	return nil
}

// signAndUpload signs each built package with the worker host key (when a signer
// is configured) and hands it to the injected Uploader with its signature.
func (s *Service) signAndUpload(ctx context.Context, repo string, packages []string) error {
	for _, pkgPath := range packages {
		info, err := os.Stat(pkgPath)
		if err != nil {
			return errors.WrapErr(err, "failed to inspect package: "+pkgPath)
		}
		if limits.Exceeds(info.Size(), s.cfg.MaxSize) {
			return fmt.Errorf("package %s exceeds max_size (%d > %d bytes)", pkgPath, info.Size(), limits.PackageBytes(s.cfg.MaxSize))
		}
		sigPath := ""
		if s.signer != nil {
			var err error
			sigPath, err = s.signer.Sign(ctx, pkgPath)
			if err != nil {
				return errors.WrapErr(err, "failed to sign package: "+pkgPath)
			}
		}

		if err := s.uploader.Upload(repo, pkgPath, sigPath); err != nil {
			return err
		}
	}
	return nil
}
