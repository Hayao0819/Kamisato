package repo

import (
	"path"

	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
)

func (p *Package) UploadToBlinky(server string, repo *SourceRepo) error {
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
