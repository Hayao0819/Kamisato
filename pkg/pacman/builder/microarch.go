package builder

import (
	"fmt"
	"strings"
)

// microarchLevels maps Arch's x86-64 feature level names to gcc -march values; other arches have no entry and reject a tier.
var microarchLevels = map[string]string{
	"x86_64_v2": "x86-64-v2",
	"x86_64_v3": "x86-64-v3",
	"x86_64_v4": "x86-64-v4",
}

// ValidMicroarch reports whether tier is empty (default build) or a known x86-64 feature level.
func ValidMicroarch(tier string) bool {
	if tier == "" {
		return true
	}
	_, ok := microarchLevels[tier]
	return ok
}

// microarchOverride returns makepkg.conf lines to pin the feature level for tier, appended after source (gcc honours the last -march).
// Returns "" for empty tier. CARCH is left as x86_64: makepkg validates against arch=(), and a v3 build is separated by repo
// (same approach as Arch's official x86-64-v3 rebuild).
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

// makepkgOverrideLines renders makepkg.conf override lines for per-build settings, appended after source so the last value wins; returns "" for zero settings.
func makepkgOverrideLines(s MakepkgSettings) (string, error) {
	var b strings.Builder
	if s.Packager != "" {
		fmt.Fprintf(&b, "PACKAGER=%s\n", shellQuote(s.Packager))
	}
	march, err := microarchOverride(s.Microarch)
	if err != nil {
		return "", err
	}
	b.WriteString(march)
	if s.CFlagsAppend != "" {
		fmt.Fprintf(&b, "CFLAGS+=%s\nCXXFLAGS+=%s\n", shellQuote(" "+s.CFlagsAppend), shellQuote(" "+s.CFlagsAppend))
	}
	if len(s.Options) > 0 {
		fmt.Fprintf(&b, "OPTIONS+=(%s)\n", strings.Join(s.Options, " "))
	}
	return b.String(), nil
}
