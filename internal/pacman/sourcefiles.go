package pacman

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MaxInlineSource caps the size of a build-dir file shipped inline. Larger files
// are assumed to be sources makepkg downloaded locally, which the builder
// re-fetches, so shipping them would only bloat the request.
const MaxInlineSource = 1 << 20 // 1 MiB

// ReadInline reads the PKGBUILD and small sidecar files from dir. Directories,
// the regenerated .SRCINFO, logs, and build outputs are skipped; a non-PKGBUILD
// file larger than MaxInlineSource is skipped after onSkipLarge (if non-nil) is
// called with its name and size.
func ReadInline(dir string, onSkipLarge func(name string, size int64)) (pkgbuild string, files map[string]string, err error) {
	return ReadInlineBuildScript(dir, "PKGBUILD", onSkipLarge)
}

func ReadInlineBuildScript(dir, buildscript string, onSkipLarge func(name string, size int64)) (pkgbuild string, files map[string]string, err error) {
	buildscript = filepath.Clean(buildscript)
	if buildscript == "." || filepath.IsAbs(buildscript) || filepath.Dir(buildscript) != "." {
		return "", nil, fmt.Errorf("build script must be a file in %s", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read source directory: %w", err)
	}

	files = map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == buildscript {
			b, rerr := os.ReadFile(filepath.Join(dir, name))
			if rerr != nil {
				return "", nil, fmt.Errorf("failed to read %s: %w", name, rerr)
			}
			pkgbuild = string(b)
			continue
		}
		if name == "PKGBUILD" {
			continue
		}
		if name == ".SRCINFO" || strings.HasSuffix(name, ".log") || IsArtifact(name) {
			continue
		}
		if info, ierr := e.Info(); ierr == nil && info.Size() > MaxInlineSource {
			if onSkipLarge != nil {
				onSkipLarge(name, info.Size())
			}
			continue
		}
		b, rerr := os.ReadFile(filepath.Join(dir, name))
		if rerr != nil {
			return "", nil, fmt.Errorf("failed to read %s: %w", name, rerr)
		}
		files[name] = string(b)
	}

	if pkgbuild == "" {
		return "", nil, fmt.Errorf("no %s found in %s", buildscript, dir)
	}
	return pkgbuild, files, nil
}
