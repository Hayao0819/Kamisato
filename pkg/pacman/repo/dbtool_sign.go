package repo

import (
	"crypto"
	"fmt"
	"io"
	"os"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// SignDetached writes a detached EdDSA/SHA-256 OpenPGP signature (matching ayato's verification keyring); the primitive shared by detachSignFile and the merged-db signer.
func SignDetached(entity *openpgp.Entity, r io.Reader, w io.Writer) error {
	return openpgp.DetachSign(w, entity, r, &packet.Config{Algorithm: packet.PubKeyAlgoEdDSA, DefaultHash: crypto.SHA256})
}

// maybeSign detach-signs .db/.files when useSignedDB and signer are both set (callers already reject the nil-signer case).
func (t NativeTool) maybeSign(paths toolPaths, useSignedDB bool) error {
	if !useSignedDB {
		return nil
	}
	if err := signAndAlias(t.signer, paths.db, paths.dbLink); err != nil {
		return err
	}
	return signAndAlias(t.signer, paths.files, paths.filesLink)
}

// signAndAlias detach-signs archivePath and also writes linkPath+".sig" (byte copy, not symlink, for blob stores) so pacman can fetch <repo>.db.sig.
func signAndAlias(entity *openpgp.Entity, archivePath, linkPath string) error {
	sigPath := archivePath + ".sig"
	if err := detachSignFile(entity, archivePath, sigPath); err != nil {
		return err
	}
	return copyToolFile(sigPath, linkPath+".sig")
}

// detachSignFile writes a detached SHA-256 OpenPGP signature of srcPath to sigPath.
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
