package stream

import (
	"io"
	"os"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/gabriel-vasile/mimetype"
)

func OpenFileWithType(filePath string) (*FileStream, error) {

	mt, err := mimetype.DetectFile(filePath)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	return NewFileStream(file.Name(), mt.String(), file), nil
}

func NewFileStreamWithType(filename string, file io.ReadSeekCloser) (*FileStream, error) {
	if file == nil {
		return nil, os.ErrInvalid
	}

	mt, file, err := utils.ReadSeekCloserWithMimeType(file)
	if err != nil {
		return nil, err
	}

	return NewFileStream(filename, mt.String(), file), nil
}
