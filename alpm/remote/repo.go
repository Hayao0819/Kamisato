package alpm

import "github.com/Hayao0819/Kamisato/alpm/pkg"

type RemoteRepo struct {
	Name string
	// Url  string
	Pkgs []*pkg.Package
}

func GetRepoFromURL(url string) (*RemoteRepo, error) {
	// TODO: implement this
	return nil, nil
}

func GetRepoFromDBFile(dbfile string) (*RemoteRepo, error) {
	return nil, nil
}
