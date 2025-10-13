// パッケージビルド関連
package pkg

import (
	"log/slog"
	"os"
	"path"

	utils "github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/pacman/gpg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/package/builder"
)

func (p *Package) Build(target *builder.Target, dest string) error {
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

	if err := target.Build(tmpdir); err != nil {
		return utils.WrapErr(err, "failed to build package")
	}

	names, err := p.PkgFileNames()
	if err != nil {
		return utils.WrapErr(err, "failed to get package file names")
	}

	if target.SignKey != "" {
		for _, name := range names {
			src := path.Join(tmpdir, name)
			if err := gpg.SignFile(target.SignKey, "", src); err != nil {
				return utils.WrapErr(err, "failed to sign file: "+src)
			}
		}
	}

	// ...（省略: ファイル移動処理など）...
	return nil
}
