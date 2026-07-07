package conf

import (
	"errors"
	"io/fs"

	"github.com/joho/godotenv"
)

// LoadEnv loads a .env file when present; a missing .env is the normal case, not
// an error.
func LoadEnv() error {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
