package buildenv

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder/internal/shellutil"
)

//go:embed makepkg.override.conf
var makepkgOverrideConf string

// microarchOverride returns makepkg.conf lines to pin the feature level for tier, appended after source (gcc honours the last -march).
// Returns "" for empty tier. CARCH is left as x86_64: makepkg validates against arch=(), and a v3 build is separated by repo
// (same approach as Arch's official x86-64-v3 rebuild).
func microarchOverride(tier string) (string, error) {
	if tier == "" {
		return "", nil
	}
	march, err := builder.MicroarchMarch(tier)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"CFLAGS+=\" -march=%s\"\nCXXFLAGS+=\" -march=%s\"\nRUSTFLAGS+=\" -Ctarget-cpu=%s\"\n",
		march, march, march,
	), nil
}

// MakepkgOverrideLines renders overrides after the base makepkg.conf.
func MakepkgOverrideLines(s builder.MakepkgConfig) (string, error) {
	var b strings.Builder
	if s.Packager != "" {
		fmt.Fprintf(&b, "PACKAGER=%s\n", shellutil.Quote(s.Packager))
	}
	march, err := microarchOverride(s.Microarch)
	if err != nil {
		return "", err
	}
	b.WriteString(march)
	if s.CFlagsAppend != "" {
		fmt.Fprintf(&b, "CFLAGS+=%s\nCXXFLAGS+=%s\n", shellutil.Quote(" "+s.CFlagsAppend), shellutil.Quote(" "+s.CFlagsAppend))
	}
	if len(s.Options) > 0 {
		options := make([]string, len(s.Options))
		for i, option := range s.Options {
			options[i] = shellutil.Quote(option)
		}
		fmt.Fprintf(&b, "OPTIONS+=(%s)\n", strings.Join(options, " "))
	}
	return b.String(), nil
}

// StageOverrideConf writes a temporary makepkg.conf for sandbox bind mounts.
func StageOverrideConf(mk builder.MakepkgConfig) (string, func(), error) {
	overrides, err := MakepkgOverrideLines(mk)
	if err != nil {
		return "", nil, err
	}
	f, err := os.CreateTemp("", "makepkg-override-*.conf")
	if err != nil {
		return "", nil, fmt.Errorf("failed to stage makepkg override: %w", err)
	}
	cleanup := func() { _ = os.Remove(f.Name()) }
	if _, err := f.WriteString(makepkgOverrideConf + overrides); err != nil {
		_ = f.Close()
		cleanup()
		return "", nil, fmt.Errorf("failed to write makepkg override: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to write makepkg override: %w", err)
	}
	return f.Name(), cleanup, nil
}
