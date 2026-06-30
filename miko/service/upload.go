package service

import (
	"context"

	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/Hayao0819/Kamisato/internal/utils"
)

// signAndUpload signs each built package with the worker host key (when a signer
// is configured) and uploads it with its detached signature to ayato.
func (s *Service) signAndUpload(ctx context.Context, repo string, packages []string) error {
	info := &blinkyutils.ServerInfo{
		URL:      s.cfg.Ayato.URL,
		Username: s.cfg.Ayato.Username,
		Password: s.cfg.Ayato.Password,
	}
	client, err := info.Client()
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

		if err := blinkyutils.Upload(client, repo, pkgPath, sigPath); err != nil {
			return utils.WrapErr(err, "failed to upload package: "+pkgPath)
		}
	}
	return nil
}
