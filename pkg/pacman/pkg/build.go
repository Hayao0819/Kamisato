package pkg

import (
	"context"
	"log/slog"
	"os"

	utils "github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	"github.com/Hayao0819/Kamisato/pkg/pacman/gpg"
)

// Build copies the SourcePackage to a temp directory, builds it, and signs it if needed.
func (p *SourcePackage) Build(target *builder.Target, dest string) error {
	var tmpdir string
	{
		var err error
		tmpdir, err = os.MkdirTemp("", "ayaka-build-*")
		if err != nil {
			return err
		}
		slog.Info("tempdir", "dir", tmpdir)
		if err := utils.CopyDir(p.dir, tmpdir); err != nil {
			return err
		}
	}
	// The output is moved to OutDir(=dest), so discard tmpdir holding the source copy.
	defer func() { _ = os.RemoveAll(tmpdir) }()

	backend, err := builder.New(builder.KindChroot, builder.Options{})
	if err != nil {
		return utils.WrapErr(err, "failed to create build backend")
	}

	result, err := backend.Build(context.Background(), builder.Spec{
		SrcDir:      tmpdir,
		OutDir:      dest,
		Arch:        target.Arch,
		ArchBuild:   target.ArchBuild,
		InstallPkgs: target.InstallPkgs,
	})
	if err != nil {
		return utils.WrapErr(err, "failed to build package")
	}

	if target.SignKey != "" {
		for _, pkgPath := range result.Packages {
			if err := gpg.SignFile(target.SignKey, "", pkgPath); err != nil {
				return utils.WrapErr(err, "failed to sign file: "+pkgPath)
			}
		}
	}

	return nil
}
