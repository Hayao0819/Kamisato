package repository

func (r *Repository) Init(name string, useSignedDB bool, gnupgDir *string) error {
	// slog.Debug("init pkg repo", "name", name)
	arches, err := r.Arches(name) // TODO: 設定ファイルから取得する
	if err != nil {
		return err
	}

	for _, arch := range arches {
		if err := r.pkgBinStore.Init(name, arch, useSignedDB, gnupgDir); err != nil {
			return err
		}
	}

	return nil
}
