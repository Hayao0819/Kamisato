package sign

import (
	"context"
	"io"
	"os"

	"github.com/Hayao0819/Kamisato/internal/filesystem/safefile"
	"github.com/ProtonMail/go-crypto/openpgp"
)

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
	if err := safefile.Replace(sigPath, 0o644, func(out io.Writer) error { //nolint:gosec // detached signatures are public repository artifacts
		return openpgp.DetachSign(out, entity, in, keyConfig())
	}); err != nil {
		return "", err
	}
	return sigPath, nil
}
