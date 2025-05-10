package repo

import (
	"fmt"
	"mime/multipart"
)

type BinRepo struct {
	// Config *conf.RepoConfig
}

type PackageBinary struct {
	file string
}

func ValidatePkgHeader(fh *multipart.FileHeader) error {

	if fh.Size == 0 {
		return fmt.Errorf("file is empty")
	}

	return nil
}
