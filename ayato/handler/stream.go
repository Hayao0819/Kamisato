package handler

import (
	"mime/multipart"

	"github.com/Hayao0819/Kamisato/ayato/repository/stream"
)

func formFileStream(f *multipart.FileHeader) (*stream.FileStream, error) {
	file, err := f.Open()
	if err != nil {
		return nil, err
	}
	return stream.NewFileStream(f.Filename, f.Header.Get("Content-Type"), file), nil
}
