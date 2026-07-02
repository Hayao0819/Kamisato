package builder

import "fmt"

// microarchLevels maps a supported x86-64 feature level (Arch's pseudo-arch
// name) to the gcc -march value that selects it. Only x86_64 has feature levels,
// so aarch64/armv7h have no entry and reject a tier.
var microarchLevels = map[string]string{
	"x86_64_v2": "x86-64-v2",
	"x86_64_v3": "x86-64-v3",
	"x86_64_v4": "x86-64-v4",
}

// ValidMicroarch reports whether tier is empty (no tier, the default build) or a
// known x86-64 feature level.
func ValidMicroarch(tier string) bool {
	if tier == "" {
		return true
	}
	_, ok := microarchLevels[tier]
	return ok
}

// microarchOverride returns the makepkg.conf lines that pin the compiler feature
// level for tier. They are meant to be appended after `source /etc/makepkg.conf`,
// so they raise the distro's baseline -march to the tier without dropping its
// other flags (gcc honours the last -march). It returns "" for an empty tier so a
// default build stays byte-for-byte unchanged, and an error for an unknown tier.
//
// CARCH is deliberately left as x86_64: makepkg validates it against the
// PKGBUILD's arch=() array, and the container platform maps from it, so a v3
// build keeps the x86_64 CARCH and is separated by repo — the same approach as
// Arch's official x86-64-v3 rebuild.
func microarchOverride(tier string) (string, error) {
	if tier == "" {
		return "", nil
	}
	march, ok := microarchLevels[tier]
	if !ok {
		return "", fmt.Errorf("unknown microarchitecture tier %q", tier)
	}
	return fmt.Sprintf(
		"CFLAGS+=\" -march=%s\"\nCXXFLAGS+=\" -march=%s\"\nRUSTFLAGS+=\" -Ctarget-cpu=%s\"\n",
		march, march, march,
	), nil
}
