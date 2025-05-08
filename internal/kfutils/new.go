package kfutils

import "github.com/knadh/koanf/v2"

// New returns a new Loader instance with a custom delimiter (default ".")
func New[T any](delimiter string) *Loader[T] {
	if delimiter == "" {
		delimiter = "."
	}
	return &Loader[T]{k: koanf.New(delimiter)}
}
