// Package keyring builds the artifacts a pacman keyring package ships: the three
// files under /usr/share/pacman/keyrings (<name>.gpg, <name>-trusted,
// <name>-revoked) and the .pkg.tar.zst that installs them plus a post-install
// hook running `pacman-key --populate <name>`. It is pure assembly over public
// key material; the private key never enters here.
package keyring

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
	"github.com/ProtonMail/go-crypto/openpgp"
)

// trustFull is gpg's ownertrust value for full trust (level 4). pacman-key
// --populate lsigns every fingerprint in the -trusted file, so a single anchor
// listed here becomes a valid signer; subkeys inherit trust through their binding
// to the anchor and need not be listed.
const trustFull = "4"

// Files are the three keyring artifacts installed under
// /usr/share/pacman/keyrings/.
type Files struct {
	Name    string // keyring identifier, e.g. "myrepo"
	GPG     []byte // <name>.gpg — concatenated binary public keys with revocation sigs
	Trusted []byte // <name>-trusted — "<fpr>:4:" lines
	Revoked []byte // <name>-revoked — bare fingerprint lines
}

// BuildFiles assembles the keyring files. entities are the public keys to bundle
// (each serialized with its revocation signatures). trustedFprs are the trust
// anchors written to -trusted (typically each key's primary fingerprint).
// revokedFprs are disabled in the local keyring by --populate; the revocation
// signatures embedded in the .gpg are the cryptographic record, the -revoked list
// is the pacman-side belt-and-suspenders.
func BuildFiles(name string, entities []*openpgp.Entity, trustedFprs, revokedFprs []string) (*Files, error) {
	if name == "" {
		return nil, fmt.Errorf("keyring name is required")
	}
	if len(entities) == 0 {
		return nil, fmt.Errorf("keyring needs at least one key")
	}

	var gpg bytes.Buffer
	for _, e := range entities {
		if err := e.Serialize(&gpg); err != nil {
			return nil, fmt.Errorf("serialize key %s: %w", sign.Fingerprint(e.PrimaryKey.Fingerprint), err)
		}
	}

	return &Files{
		Name:    name,
		GPG:     gpg.Bytes(),
		Trusted: fingerprintLines(trustedFprs, ":"+trustFull+":"),
		Revoked: fingerprintLines(revokedFprs, ""),
	}, nil
}

// fingerprintLines renders one uppercase fingerprint per line with an optional
// suffix, deduplicated and sorted for a reproducible file.
func fingerprintLines(fprs []string, suffix string) []byte {
	seen := make(map[string]struct{}, len(fprs))
	uniq := make([]string, 0, len(fprs))
	for _, f := range fprs {
		f = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(f), " ", ""))
		if f == "" {
			continue
		}
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		uniq = append(uniq, f)
	}
	sort.Strings(uniq)

	var b strings.Builder
	for _, f := range uniq {
		b.WriteString(f)
		b.WriteString(suffix)
		b.WriteByte('\n')
	}
	return []byte(b.String())
}
