package repository

import (
	"fmt"

	utils "github.com/Hayao0819/Kamisato/internal"
)

func (r *Repository) Init(name string, useSignedDB bool, gnupgDir *string) error {

	createdArches, err := r.Arches(name)
	if err != nil {
		createdArches = []string{}
	}

	repoconfig := r.cfg.Repo(name)
	if repoconfig == nil {
		return fmt.Errorf("repository %s not found in config", name)
	}

	arches := utils.Merge(createdArches, repoconfig.Arches)

	for _, arch := range arches {
		if err := r.pkgBinStore.Init(name, arch, useSignedDB, gnupgDir); err != nil {
			return err
		}
	}

	return nil
}
