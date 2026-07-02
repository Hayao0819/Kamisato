package stream

import (
	"os"

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
