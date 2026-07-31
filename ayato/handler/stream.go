package handler

import (
	"mime/multipart"

	"github.com/Hayao0819/Kamisato/ayato/blob"
)

func formFileStream(f *multipart.FileHeader) (*blob.FileStream, error) {
	file, err := f.Open()
	if err != nil {
		return nil, err
	}
	return blob.NewFileStream(f.Filename, f.Header.Get("Content-Type"), file), nil
}
