package stream

import (
	"io"
	"os"

	"github.com/gabriel-vasile/mimetype"
)

func OpenFileStreamWithTypeDetection(filePath string) (*FileStream, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	mt, err := mimetype.DetectReader(file)
	if err != nil {
		return nil, err
	}
	file.Seek(0, io.SeekStart) // Reset the file pointer to the beginning

	return NewFileStream(file.Name(), mt.String(), file), nil
}
