package sign

import (
	"fmt"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// Fingerprint renders an OpenPGP fingerprint as uppercase hex, the form pacman's
// keyring `-trusted`/`-revoked` files use.
func Fingerprint(fpr []byte) string {
	return strings.ToUpper(fmt.Sprintf("%x", fpr))
}

// normalizeFingerprint strips spaces and an optional 0x prefix and uppercases, so
// a user can paste a spaced or lowercase fingerprint.
func normalizeFingerprint(s string) string {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	return strings.ToUpper(s)
}

// ParseRevocationReason maps a user-facing reason word to its OpenPGP code. The
// hard/soft distinction is the caller's concern (see IsHardRevocation): soft
// reasons keep pre-revocation signatures valid, hard ones invalidate them.
func ParseRevocationReason(s string) (packet.ReasonForRevocation, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "superseded":
		return packet.KeySuperseded, nil
	case "retired", "no-longer-used":
		return packet.KeyRetired, nil
	case "compromised":
		return packet.KeyCompromised, nil
	case "unspecified", "":
		return packet.NoReason, nil
	default:
		return packet.NoReason, fmt.Errorf("unknown revocation reason %q (want superseded|retired|compromised|unspecified)", s)
	}
}

// IsHardRevocation reports whether a reason retroactively invalidates signatures
// made before the revocation. compromised and unspecified are hard; superseded
// and retired are soft.
func IsHardRevocation(reason packet.ReasonForRevocation) bool {
	return reason == packet.KeyCompromised || reason == packet.NoReason
}
