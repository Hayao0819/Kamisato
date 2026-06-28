package conf

import (
	"errors"
	"io/fs"

	"github.com/joho/godotenv"
)

// LoadEnv loads a .env file when one is present. A missing .env is the normal
// case, not an error, so callers don't log noise on every invocation.
func LoadEnv() error {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
