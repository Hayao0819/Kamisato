package repo

import (
	"bytes"
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

func (p *Package) Build(method string, target *abs.Target) error {
	builder := abs.GetBuilder(method)
	logger.Info("Build %s", p.Path)
	return builder.Build(p.Path, target)
}

func (p *Package) GetPkgFilePath() string {
	stdout := new(bytes.Buffer)
	cmd := exec.Command("makepkg", "--packagelist")
	cmd.Dir = p.Path
	cmd.Stdout = stdout
	err := cmd.Run()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(stdout.String())
}

func (p *Package) MovePkgFile(dst string) error {
	src := p.GetPkgFilePath()
	logger.Info("Move %s to %s", src, dst)
	return utils.MoveFile(src, dst)
}

func (p *Package) UploadToBlinky(server string, repo *Repository) error {
	client, error := blinkyutils.GetClient(server)
	if error != nil {
		return error
	}

	return client.UploadPackageFiles(repo.Config.Name, p.GetPkgFilePath())
}
