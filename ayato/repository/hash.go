package repository

import (
	"crypto/sha256"
	"io"
)

func hashReader(reader io.Reader) ([sha256.Size]byte, error) {
	hasher := sha256.New()
	if _, err := io.Copy(hasher, reader); err != nil {
		return [sha256.Size]byte{}, err
	}
	var sum [sha256.Size]byte
	copy(sum[:], hasher.Sum(nil))
	return sum, nil
}
