package kfutils

import (
	"github.com/knadh/koanf/v2"
)

type Loader[T any] struct {
	k         *koanf.Koanf
	dirs      []string
	filenames []string
}

func (l *Loader[T]) Get() (*T, error) {
	var cfg T
	err := l.Unmarshal(&cfg)
	return &cfg, err
}

func (l *Loader[T]) Unmarshal(v *T) error {
	return l.k.Unmarshal("", v)
}
