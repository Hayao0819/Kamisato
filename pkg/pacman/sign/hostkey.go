package sign

import (
	"context"
	"os"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// HostKeySigner signs with the worker host key from a Keystore.
type HostKeySigner struct{ worker *openpgp.Entity }

func NewHostKeySigner(k *Keystore) *HostKeySigner { return &HostKeySigner{worker: k.WorkerEntity()} }

func (s *HostKeySigner) Sign(ctx context.Context, pkgPath string) (string, error) {
	return detachSign(ctx, s.worker, pkgPath)
}

func detachSign(ctx context.Context, entity *openpgp.Entity, pkgPath string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	in, err := os.Open(pkgPath)
	if err != nil {
		return "", err
	}
	defer func() { _ = in.Close() }()

	sigPath := pkgPath + ".sig"
	out, err := os.Create(sigPath)
	if err != nil {
		return "", err
	}
	defer func() { _ = out.Close() }()

	if err := openpgp.DetachSign(out, entity, in, keyConfig()); err != nil {
		_ = os.Remove(sigPath)
		return "", err
	}
	return sigPath, nil
}
