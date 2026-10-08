package service

import (
	"context"
	"log/slog"

	"github.com/Hayao0819/Kamisato/internal/pacman/sign"
)

// SignerRegistrar is the certificate-registration operation required at startup.
type SignerRegistrar interface {
	RegisterSigner(context.Context, []byte) (string, error)
}

// RegisterWorkerCert registers the worker signing certificate through registrar.
func RegisterWorkerCert(ctx context.Context, registrar SignerRegistrar, keystore *sign.Keystore) error {
	certificate, err := keystore.WorkerCertArmored()
	if err != nil {
		return err
	}
	fingerprint, err := registrar.RegisterSigner(ctx, []byte(certificate))
	if err != nil {
		return err
	}
	slog.Info("registered worker signing key with ayato", "fingerprint", fingerprint)
	return nil
}
