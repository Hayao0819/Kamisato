// Package keyring builds the artifacts a pacman keyring package ships: the three
// files under /usr/share/pacman/keyrings (<name>.gpg, <name>-trusted,
// <name>-revoked) and the .pkg.tar.zst that installs them plus a post-install
// hook running `pacman-key --populate <name>`. It is pure assembly over public
// key material; the private key never enters here.
package keyring

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Hayao0819/Kamisato/pkg/atomicfile"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
	"github.com/ProtonMail/go-crypto/openpgp"
)

// nameRe constrains a keyring name to a safe pkgname/path/arg; rejects slashes, whitespace,
// and shell metacharacters to prevent directory escape in the tarball or injection into the install hook.
var nameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9@._+-]*$`)

// ValidateName reports whether name is usable as a keyring identifier.
func ValidateName(name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("invalid keyring name %q: use letters, digits and @._+- (no slashes or spaces)", name)
	}
	return nil
}

// trustFull is gpg's ownertrust level 4; pacman-key --populate lsigns every -trusted fingerprint,
// so subkeys inherit trust through their anchor binding and need not be listed separately.
const trustFull = "4"

// Files are the three keyring artifacts installed under
// /usr/share/pacman/keyrings/.
type Files struct {
	Name    string // keyring identifier, e.g. "myrepo"
	GPG     []byte // <name>.gpg — concatenated binary public keys with revocation sigs
	Trusted []byte // <name>-trusted — "<fpr>:4:" lines
	Revoked []byte // <name>-revoked — bare fingerprint lines
}

// Write emits the three keyring files into dir, leaving repo packaging untouched
// so ayaka can regenerate key material in place.
func (f *Files) Write(dir string) error {
	// #nosec G301 -- pacman keyrings contain only public keys and trust metadata.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	items := []struct {
		name string
		data []byte
	}{
		{f.Name + ".gpg", f.GPG},
		{f.Name + "-trusted", f.Trusted},
		{f.Name + "-revoked", f.Revoked},
	}
	for _, it := range items {
		// #nosec G306 -- these public keyring files must be readable by pacman users.
		if err := atomicfile.WriteFile(filepath.Join(dir, it.name), it.data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// BuildFiles assembles the three keyring artifacts; revokedFprs are disabled by --populate
// (the -revoked list is belt-and-suspenders alongside the revocation sigs embedded in the .gpg).
func BuildFiles(name string, entities []*openpgp.Entity, trustedFprs, revokedFprs []string) (*Files, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
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

// fingerprintLines renders deduplicated, sorted uppercase fingerprints one per line with an optional suffix.
func fingerprintLines(fprs []string, suffix string) []byte {
	seen := make(map[string]struct{}, len(fprs))
	uniq := make([]string, 0, len(fprs))
	for _, f := range fprs {
		f = sign.NormalizeFingerprint(f)
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
