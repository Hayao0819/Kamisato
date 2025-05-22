package localfs

import (
	"io"
	"os"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/gabriel-vasile/mimetype"
)

func openFileStreamWithTypeDetection(filePath string) (*domain.FileStream, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	mt, err := mimetype.DetectReader(file)
	if err != nil {
		return nil, err
	}
	file.Seek(0, io.SeekStart) // Reset the file pointer to the beginning

	return domain.NewFileStream(file.Name(), mt.String(), file), nil
}
