package stream

import (
	"io"
	"os"

	"github.com/gabriel-vasile/mimetype"
)

func Rewind(seeker io.Seeker) error {
	_, err := seeker.Seek(0, io.SeekStart)
	return err
}

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
