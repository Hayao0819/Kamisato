package repo

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/Hayao0819/Kamisato/ayaka/abs"
	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Morganamilo/go-srcinfo"
	"github.com/samber/lo"
)

type Package struct {
	Path    string
	Srcinfo *srcinfo.Srcinfo
}

func (p *Package) Names() []string {
	names := []string{
		p.Srcinfo.Pkgbase,
	}
	for _, pkg := range p.Srcinfo.Packages {
		names = append(names, pkg.Pkgname)
	}

	names = lo.Uniq(names)

	return names
}

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
		if err := utils.CopyDir(p.Path, tmpdir); err != nil {
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

func (p *Package) GetPkgFileNames() ([]string, error) {
	stdout := new(bytes.Buffer)
	cmd := exec.Command("makepkg", "--packagelist")
	cmd.Dir = p.Path
	cmd.Stdout = stdout
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	l := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(l) == 0 {
		return nil, fmt.Errorf("no package found")
	}
	pkgs := make([]string, len(l))
	for i, pkg := range l {
		pkgs[i] = path.Base(strings.TrimSpace(pkg))
	}

	return pkgs, nil
}

func (p *Package) UploadToBlinky(server string, repo *Repository) error {
	client, err := blinkyutils.GetClient(server)
	if err != nil {
		return err
	}

	fp, err := p.GetPkgFileNames()
	if err != nil {
		return err
	}

	fullpaths := make([]string, len(fp))
	for i, f := range fp {
		fullpaths[i] = path.Join(p.Path, f)
	}

	return client.UploadPackageFiles(repo.Config.Name, fullpaths...)
}
