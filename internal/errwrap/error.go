package errwrap

import (
	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/errors/errbase"
)

func hasStack(err error) bool {
	if err == nil {
		return false
	}

	// whether this is a cockroachdb/errors withStack
	_, ok := err.(interface{ SafeFormatError(errbase.Printer) error })
	return ok
}

func WrapErr(err error, msg string) error {
	if err == nil {
		return nil
	}

	if hasStack(err) {
		return errors.WithMessage(err, msg)
	}

	return errors.Wrap(err, msg)
}

func NewErr(msg string) error {
	return errors.New(msg)
}

func NewErrf(format string, args ...interface{}) error {
	return errors.Newf(format, args...)
}
