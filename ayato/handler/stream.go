package handler

import (
	"mime/multipart"

	"github.com/Hayao0819/Kamisato/ayato/domain"
)

func formFileStream(f *multipart.FileHeader) (domain.IFileStream, error) {
	file, err := f.Open()
	if err != nil {
		return nil, err
	}
	return domain.NewFileStream(f.Filename, f.Header.Get("Content-Type"), file), nil
}
