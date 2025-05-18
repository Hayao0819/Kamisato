package alpm

import (
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/alpm/builder"
	"github.com/Hayao0819/Kamisato/ayaka/gpg"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/nahi/futils"
	"github.com/cockroachdb/errors"
)

func (p *Package) Build(target *builder.Target, dest string) error {
	builder := builder.Determine(target)
	if builder == nil {
		return fmt.Errorf("no builder found for target %s", target.Arch)
	}

	var tmpdir string
	{
		// Create temp directory
		var err error
		tmpdir, err = os.MkdirTemp("", "ayaka-build-*")
		if err != nil {
			return err
		}
		slog.Info("tempdir", "dir", tmpdir)
		// defer os.RemoveAll(tmpdir)

		// Copy directory to temp directory
		if err := utils.CopyDir(p.path, tmpdir); err != nil {
			return err
		}
	}

	// Build package
	if err := builder.Build(tmpdir, target); err != nil {
		return errors.Wrap(err, "failed to build package")
	}

	// Move files
	names, err := p.GetPkgFileNames()
	if err != nil {
		return errors.Wrap(err, "failed to get package file names")
	}

	// Sign files
	if target.SignKey != "" {
		for _, name := range names {
			src := path.Join(tmpdir, name)
			if err := gpg.SignFile(target.SignKey, "", src); err != nil {
				return errors.Wrap(err, "failed to sign file: "+src)
			}
		}
	}

	for _, name := range names {
		src := path.Join(tmpdir, name)
		if err := utils.MoveFile(src, dest); err != nil {
			return err
		}
		if futils.Exists(src + ".sig") {
			if err := utils.MoveFile(src+".sig", dest); err != nil {
				return err
			}
		}
	}
	return nil
}
