package repo

import (
	"fmt"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/ayaka/abs"
	"github.com/Hayao0819/Kamisato/internal/utils"
)

func (p *Package) Build(method string, target *abs.Target, dest string) error {
	builder := abs.GetBuilder(method)
	if builder == nil {
		return fmt.Errorf("unknown build method %s", method)
	}

	var tmpdir string
	{
		// Create temp directory
		var err error
		tmpdir, err = os.MkdirTemp("", "ayaka-build-*")
		if err != nil {
			return err
		}
		// defer os.RemoveAll(tmpdir)

		// Copy directory to temp directory
		if err := utils.CopyDir(path.Dir(p.path), tmpdir); err != nil {
			return err
		}
	}

	// Build package
	if err := builder.Build(tmpdir, target); err != nil {
		return err
	}

	// Move files
	names, err := p.GetPkgFileNames()
	if err != nil {
		return err
	}
	for _, name := range names {
		src := path.Join(tmpdir, name)
		if err := utils.MoveFile(src, dest); err != nil {
			return err
		}
	}
	return nil
}
