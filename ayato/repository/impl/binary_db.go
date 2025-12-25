package impl

import "github.com/Hayao0819/Kamisato/ayato/stream"

// FetchDB retrieves the DB file for the given repository and architecture.
func (r *PackageBinaryRepository) FetchDB(repoName, archName string) (stream.IFileStream, error) {
	db, err := r.pkgBinStore.FetchFile(repoName, archName, repoName+".db")
	if err != nil {
		return nil, err
	}
	return db, nil
}
