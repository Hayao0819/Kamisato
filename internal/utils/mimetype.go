package utils

import (
	"io"

	"github.com/gabriel-vasile/mimetype"
)

func ReadSeekerWithMimeType[T io.ReadSeeker](input T) (*mimetype.MIME, T, error) {
	mtype, err := mimetype.DetectReader(input)
	if err != nil {
		return nil, input, err
	}

	_, err = input.Seek(0, io.SeekStart)
	if err != nil {
		return mtype, input, err
	}

	return mtype, input, nil
}
