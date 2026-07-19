package handler

import (
	"mime/multipart"

	"github.com/Hayao0819/Kamisato/ayato/platform"
)

func formFileStream(f *multipart.FileHeader) (*platform.FileStream, error) {
	file, err := f.Open()
	if err != nil {
		return nil, err
	}
	return platform.NewFileStream(f.Filename, f.Header.Get("Content-Type"), file), nil
}
