package repo

import (
	"fmt"
	"mime/multipart"
)

func ValidatePkgHeader(fh *multipart.FileHeader) error {

	if fh.Size == 0 {
		return fmt.Errorf("file is empty")
	}

	return nil
}
