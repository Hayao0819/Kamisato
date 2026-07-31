package service

import (
	"context"
	"log/slog"

	"github.com/Hayao0819/Kamisato/internal/ayatoapi"
	"github.com/Hayao0819/Kamisato/internal/pacman/sign"
)

// RegisterWorkerCert registers the worker signing certificate with Ayato.
func RegisterWorkerCert(ctx context.Context, settings Settings, keystore *sign.Keystore) error {
	certificate, err := keystore.WorkerCertArmored()
	if err != nil {
		return err
	}
	publisher, err := ayatoapi.NewPublisher(settings.AyatoURL, settings.AyatoAPIKey)
	if err != nil {
		return err
	}
	fingerprint, err := publisher.RegisterSigner(ctx, []byte(certificate))
	if err != nil {
		return err
	}
	slog.Info("registered worker signing key with ayato", "url", settings.AyatoURL, "fingerprint", fingerprint)
	return nil
}
