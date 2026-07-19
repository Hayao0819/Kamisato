package service

import (
	"context"
	"fmt"
	"os"

	"github.com/Hayao0819/Kamisato/internal/client"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/limits"
)

type PackageUpload struct {
	PackagePath   string
	SignaturePath string
}

// Uploader publishes build artifacts to Ayato.
type Uploader interface {
	Upload(ctx context.Context, repo string, packages []PackageUpload) error
}

type ayatoUploader struct {
	client *client.Publisher
}

func NewAyatoUploader(rawURL, apiKey string) (Uploader, error) {
	if rawURL == "" {
		return nil, nil
	}
	publisher, err := client.NewPublisher(rawURL, apiKey)
	if err != nil {
		return nil, err
	}
	return &ayatoUploader{client: publisher}, nil
}

func (u *ayatoUploader) Upload(ctx context.Context, repo string, packages []PackageUpload) error {
	files := make([]string, 0, len(packages)*2)
	for _, item := range packages {
		files = append(files, item.PackagePath)
		if item.SignaturePath != "" {
			files = append(files, item.SignaturePath)
		}
	}
	if err := u.client.UploadPackageFiles(ctx, repo, files...); err != nil {
		return errors.WrapErr(err, "publish build result")
	}
	return nil
}

// signAndUpload validates, signs, and publishes a build result.
func (s *Service) signAndUpload(ctx context.Context, repo string, packages []string) error {
	if s.uploader == nil {
		return errors.NewErr("Ayato publisher is not configured")
	}
	for _, pkgPath := range packages {
		info, err := os.Stat(pkgPath)
		if err != nil {
			return errors.WrapErr(err, "failed to inspect package: "+pkgPath)
		}
		if limits.Exceeds(info.Size(), s.cfg.MaxSize) {
			return fmt.Errorf("package %s exceeds max_size (%d > %d bytes)", pkgPath, info.Size(), limits.PackageBytes(s.cfg.MaxSize))
		}
	}

	batch := make([]PackageUpload, 0, len(packages))
	for _, pkgPath := range packages {
		sigPath := ""
		if s.signer != nil {
			var err error
			sigPath, err = s.signer.Sign(ctx, pkgPath)
			if err != nil {
				return errors.WrapErr(err, "failed to sign package: "+pkgPath)
			}
		}

		batch = append(batch, PackageUpload{PackagePath: pkgPath, SignaturePath: sigPath})
	}
	return s.uploader.Upload(ctx, repo, batch)
}
