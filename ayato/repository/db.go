package repository

import "github.com/Hayao0819/Kamisato/ayato/stream"

func (r *Repository) FetchDB(repoName, archName string) (stream.IFileStream, error) {
	db, err := r.pkgBinStore.FetchFile(repoName, archName, repoName+".db")
	if err != nil {
		return nil, err
	}

	return db, nil
}
