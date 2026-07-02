// Package srcpkg reads a source package's inline files (PKGBUILD and small
// sidecars) to ship to a remote builder. Build outputs, the regenerated
// .SRCINFO, logs, and large downloaded sources are skipped so the request stays
// small and the builder re-fetches sources from the PKGBUILD itself.
package srcpkg

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/utils"
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
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil, utils.WrapErr(err, "failed to read source directory")
	}

	files = map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == ".SRCINFO" || strings.HasSuffix(name, ".log") || strings.Contains(name, ".pkg.tar") {
			continue
		}
		if info, ierr := e.Info(); ierr == nil && name != "PKGBUILD" && info.Size() > MaxInlineSource {
			if onSkipLarge != nil {
				onSkipLarge(name, info.Size())
			}
			continue
		}
		b, rerr := os.ReadFile(filepath.Join(dir, name))
		if rerr != nil {
			return "", nil, utils.WrapErr(rerr, "failed to read "+name)
		}
		if name == "PKGBUILD" {
			pkgbuild = string(b)
			continue
		}
		files[name] = string(b)
	}

	if pkgbuild == "" {
		return "", nil, utils.NewErr("no PKGBUILD found in " + dir)
	}
	return pkgbuild, files, nil
}
