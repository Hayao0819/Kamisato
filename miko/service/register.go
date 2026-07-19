package service

import (
	"context"
	"log/slog"

	"github.com/Hayao0819/Kamisato/internal/client"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

// RegisterWorkerCert registers the worker signing certificate with Ayato.
func RegisterWorkerCert(ctx context.Context, cfg *conf.MikoConfig, keystore *sign.Keystore) error {
	certificate, err := keystore.WorkerCertArmored()
	if err != nil {
		return err
	}
	publisher, err := client.NewPublisher(cfg.Ayato.URL, cfg.Ayato.APIKey)
	if err != nil {
		return err
	}
	fingerprint, err := publisher.RegisterSigner(ctx, []byte(certificate))
	if err != nil {
		return err
	}
	slog.Info("registered worker signing key with ayato", "url", cfg.Ayato.URL, "fingerprint", fingerprint)
	return nil
}
