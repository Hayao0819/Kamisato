package source

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path"

	"github.com/Hayao0819/Kamisato/internal/filesystem/safefile"
)

// GenerateSrcinfo rewrites <dir>/.SRCINFO from the PKGBUILD in dir by running
// `makepkg --printsrcinfo`. makepkg must be on PATH. The output is buffered and
// written only after makepkg succeeds, so a failing PKGBUILD leaves the existing
// .SRCINFO intact instead of truncating it.
func GenerateSrcinfo(ctx context.Context, dir string, stderr io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var buf bytes.Buffer
	gencmd := exec.CommandContext(ctx, "makepkg", "--printsrcinfo")
	gencmd.Dir = dir
	gencmd.Stdout = &buf
	gencmd.Stderr = stderr
	if err := gencmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("generate .SRCINFO in %s: %w", dir, err)
	}

	if err := safefile.WriteFile(path.Join(dir, ".SRCINFO"), buf.Bytes(), 0o644); err != nil { //nolint:gosec // .SRCINFO is world-readable repo metadata
		return fmt.Errorf("write .SRCINFO in %s: %w", dir, err)
	}
	return nil
}
