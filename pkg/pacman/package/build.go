// パッケージビルド関連
package pkg

import (
	"fmt"
	"log/slog"
	"os"
	"path"

	utils "github.com/Hayao0819/Kamisato/internal"
	"github.com/Hayao0819/Kamisato/pkg/pacman/gpg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/package/builder"
	"github.com/cockroachdb/errors"
)

func (p *Package) Build(target *builder.Target, dest string) error {
	builder := builder.Determine(target)
	if builder == nil {
		return fmt.Errorf("no builder found for target %s", target.Arch)
	}

	var tmpdir string
	{
		var err error
		tmpdir, err = os.MkdirTemp("", "ayaka-build-*")
		if err != nil {
			return err
		}
		slog.Info("tempdir", "dir", tmpdir)
		if err := utils.CopyDir(p.srcdir, tmpdir); err != nil {
			return err
		}
	}

	if err := builder.Build(tmpdir, target); err != nil {
		return errors.Wrap(err, "failed to build package")
	}

	names, err := p.GetPkgFileNames()
	if err != nil {
		return errors.Wrap(err, "failed to get package file names")
	}

	if target.SignKey != "" {
		for _, name := range names {
			src := path.Join(tmpdir, name)
			if err := gpg.SignFile(target.SignKey, "", src); err != nil {
				return errors.Wrap(err, "failed to sign file: "+src)
			}
		}
	}

	// ...（省略: ファイル移動処理など）...
	return nil
}
