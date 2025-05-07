package repo

import (
	"bytes"
	"os/exec"
	"strings"

	builder "github.com/Hayao0819/Kamisato/abs"
	"github.com/Hayao0819/Kamisato/logger"
	"github.com/Hayao0819/Kamisato/utils"
	blinky_utils "github.com/Hayao0819/Kamisato/utils/blinky"
	"github.com/Morganamilo/go-srcinfo"
)

type Package struct {
	Path    string
	Srcinfo *srcinfo.Srcinfo
}

func (p *Package) Build(method string, target *builder.Target) error {
	builder := builder.GetBuilder(method)
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
	client, error := blinky_utils.GetClient(server)
	if error != nil {
		return error
	}

	return client.UploadPackageFiles(repo.Config.Name, p.GetPkgFilePath())
}
