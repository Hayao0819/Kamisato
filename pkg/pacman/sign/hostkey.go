package sign

import (
	"context"
	"io"
	"os"

	"github.com/Hayao0819/Kamisato/pkg/atomicfile"
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
	if err := atomicfile.Replace(sigPath, 0o644, func(out io.Writer) error { //nolint:gosec // detached signatures are public repository artifacts
		return openpgp.DetachSign(out, entity, in, keyConfig())
	}); err != nil {
		return "", err
	}
	return sigPath, nil
}
