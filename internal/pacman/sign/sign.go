// Package sign abstracts producing OpenPGP detached signatures for built
// packages. LocalSigner accepts either a certified worker key or an existing
// local user key; SigningKey additionally enforces its rotatable-subkey model.
package sign

import "context"

// Signer writes a detached binary OpenPGP signature for pkgPath, returning the
// signature path (conventionally pkgPath + ".sig").
type Signer interface {
	Sign(ctx context.Context, pkgPath string) (sigPath string, err error)
}
