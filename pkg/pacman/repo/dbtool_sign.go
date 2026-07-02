package repo

import (
	"crypto"
	"fmt"
	"io"
	"os"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// SignDetached writes a detached OpenPGP signature of r to w, using EdDSA over
// SHA-256 to match the digests ayato's verification keyring accepts. It is the
// byte-level primitive the file-based detachSignFile and the merged-db signer
// share.
func SignDetached(entity *openpgp.Entity, r io.Reader, w io.Writer) error {
	return openpgp.DetachSign(w, entity, r, &packet.Config{Algorithm: packet.PubKeyAlgoEdDSA, DefaultHash: crypto.SHA256})
}

// maybeSign detach-signs the .db/.files archives when a signed database is
// requested. RepoAddBatch/RepoRemove already reject useSignedDB with a nil signer,
// so this is a no-op unless both are set.
func (t NativeTool) maybeSign(paths toolPaths, useSignedDB bool) error {
	if !useSignedDB {
		return nil
	}
	if err := signAndAlias(t.signer, paths.db, paths.dbLink); err != nil {
		return err
	}
	return signAndAlias(t.signer, paths.files, paths.filesLink)
}

// signAndAlias detach-signs archivePath into archivePath+".sig" and, mirroring the
// bare <repo>.db byte-copy alias, also writes linkPath+".sig" (a blob store has no
// symlinks) so a pacman client that fetches <repo>.db.sig finds it.
func signAndAlias(entity *openpgp.Entity, archivePath, linkPath string) error {
	sigPath := archivePath + ".sig"
	if err := detachSignFile(entity, archivePath, sigPath); err != nil {
		return err
	}
	return copyToolFile(sigPath, linkPath+".sig")
}

// detachSignFile writes a detached OpenPGP signature of srcPath to sigPath, using
// SHA-256 to match the digests ayato's verification keyring accepts.
func detachSignFile(entity *openpgp.Entity, srcPath, sigPath string) error {
	in, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open %s for signing: %w", srcPath, err)
	}
	defer in.Close()
	out, err := os.Create(sigPath)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", sigPath, err)
	}
	if err := SignDetached(entity, in, out); err != nil {
		_ = out.Close()
		return fmt.Errorf("failed to sign %s: %w", srcPath, err)
	}
	return out.Close()
}
