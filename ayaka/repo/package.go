package repo

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Hayao0819/Kamisato/ayaka/abs"
	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/Hayao0819/Kamisato/internal/logger"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Morganamilo/go-srcinfo"
)

type Package struct {
	Path    string
	Srcinfo *srcinfo.Srcinfo
}

func (p *Package) Build(method string, target *abs.Target, dest string) error {
	builder := abs.GetBuilder(method)
	if builder == nil {
		return fmt.Errorf("unknown build method %s", method)
	}

	if err := builder.Build(p.Path, target); err != nil {
		return err
	}

	if err := p.movePkgFile(dest); err != nil {
		return err
	}

	return nil
}

func (p *Package) GetPkgFilePath() (string, error) {
	stdout := new(bytes.Buffer)
	cmd := exec.Command("makepkg", "--packagelist")
	cmd.Dir = p.Path
	cmd.Stdout = stdout
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (p *Package) movePkgFile(dst string) error {
	src, err := p.GetPkgFilePath()
	if err != nil {
		return err
	}
	logger.Info("Move %s to %s", src, dst)
	return utils.MoveFile(src, dst)
}

func (p *Package) UploadToBlinky(server string, repo *Repository) error {
	client, err := blinkyutils.GetClient(server)
	if err != nil {
		return err
	}

	fp, err := p.GetPkgFilePath()
	if err != nil {
		return err
	}

	return client.UploadPackageFiles(repo.Config.Name, fp)
}
