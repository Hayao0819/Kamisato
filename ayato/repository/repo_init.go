package repository

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/utils"
)

func (r *Repository) Init(name string, useSignedDB bool, gnupgDir *string) error {

	// slog.Debug("init pkg repo", "name", name)
	createdArches, err := r.Arches(name) // TODO: 設定ファイルから取得する
	if err != nil {
		return err
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
