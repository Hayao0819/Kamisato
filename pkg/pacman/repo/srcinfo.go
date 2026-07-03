package repo

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path"

	"github.com/Hayao0819/Kamisato/internal/errwrap"
)

// GenerateSrcinfo rewrites <dir>/.SRCINFO from the PKGBUILD in dir by running
// `makepkg --printsrcinfo`. makepkg must be on PATH. The output is buffered and
// written only after makepkg succeeds, so a failing PKGBUILD leaves the existing
// .SRCINFO intact instead of truncating it.
func GenerateSrcinfo(dir string, stderr io.Writer) error {
	var buf bytes.Buffer
	gencmd := exec.Command("makepkg", "--printsrcinfo")
	gencmd.Dir = dir
	gencmd.Stdout = &buf
	gencmd.Stderr = stderr
	if err := gencmd.Run(); err != nil {
		return errwrap.WrapErr(err, "generate .SRCINFO in "+dir)
	}

	if err := os.WriteFile(path.Join(dir, ".SRCINFO"), buf.Bytes(), 0o644); err != nil { //nolint:gosec // .SRCINFO is world-readable repo metadata
		return errwrap.WrapErr(err, "write .SRCINFO in "+dir)
	}
	return nil
}
