package utils

import (
	"bytes"
	"io"

	"github.com/gabriel-vasile/mimetype"
	"github.com/jarxorg/io2"
)

func ReaderWithMimeType(input io.Reader) (*mimetype.MIME, io.Reader, error) {
	header := bytes.NewBuffer(nil)
	mtype, err := mimetype.DetectReader(io.TeeReader(input, header))
	if err != nil {
		return nil, input, err
	}

	recycled := io.MultiReader(header, input)

	return mtype, recycled, err
}

func ReadCloser(input io.ReadCloser) (*mimetype.MIME, io.ReadCloser, error) {
	header := bytes.NewBuffer(nil)
	mtype, err := mimetype.DetectReader(io.TeeReader(input, header))
	if err != nil {
		return nil, input, err
	}
	recycled := io2.NewMultiReadCloser(BufferToReadSeekCloser(header), input)
	return mtype, recycled, nil
}

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
