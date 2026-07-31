// Package sign abstracts producing OpenPGP detached signatures for built
// packages. Two backends implement Signer: a worker host key certified by a
// shared master (HostKeySigner) and an arbitrary local user key (LocalSigner).
package sign

import "context"

// Signer writes a detached binary OpenPGP signature for pkgPath, returning the
// signature path (conventionally pkgPath + ".sig").
type Signer interface {
	Sign(ctx context.Context, pkgPath string) (sigPath string, err error)
}
