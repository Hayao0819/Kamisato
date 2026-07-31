package config

import (
	"fmt"
	"time"

	"github.com/knadh/koanf/v2"
)

func ValidateDuration(source *koanf.Koanf, path string) error {
	if !source.Exists(path) {
		return nil
	}
	value := source.Get(path)
	if duration, ok := value.(time.Duration); ok {
		if duration < 0 {
			return fmt.Errorf("%s must not be negative", path)
		}
		return nil
	}
	text, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s must be a duration string such as \"30m\", got %T", path, value)
	}
	duration, err := time.ParseDuration(text)
	if err != nil {
		return fmt.Errorf("%s %q is not a valid duration: %w", path, text, err)
	}
	if duration < 0 {
		return fmt.Errorf("%s must not be negative", path)
	}
	return nil
}
