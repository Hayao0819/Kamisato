package conf

import (
	"io/fs"

	"github.com/Hayao0819/Kamisato/internal/errors"

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
