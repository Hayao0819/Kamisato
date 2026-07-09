// Package errors is the application layer's single error package: every package
// outside pkg/ imports this instead of the standard library's errors or
// cockroachdb/errors, so the wrapping backend lives in one small, swappable
// place. pkg/ deliberately stays on the standard library to keep the reusable
// layer dependency-light.
package errors

import (
	stderrors "errors"

	crdb "github.com/cockroachdb/errors"
	"github.com/cockroachdb/errors/errbase"
)

// New, Is, As, Join, Unwrap, and ErrUnsupported mirror the standard library's
// errors package so callers never import "errors" directly.
func New(text string) error         { return stderrors.New(text) }
func Is(err, target error) bool     { return stderrors.Is(err, target) }
func As(err error, target any) bool { return stderrors.As(err, target) }
func Join(errs ...error) error      { return stderrors.Join(errs...) }
func Unwrap(err error) error        { return stderrors.Unwrap(err) }

// ErrUnsupported mirrors errors.ErrUnsupported.
var ErrUnsupported = stderrors.ErrUnsupported

func hasStack(err error) bool {
	if err == nil {
		return false
	}
	// whether this is a cockroachdb/errors withStack
	_, ok := err.(interface{ SafeFormatError(errbase.Printer) error })
	return ok
}

// WrapErr adds msg to err, carrying a stack trace via cockroachdb/errors.
func WrapErr(err error, msg string) error {
	if err == nil {
		return nil
	}
	if hasStack(err) {
		return crdb.WithMessage(err, msg)
	}
	return crdb.Wrap(err, msg)
}

// NewErr and NewErrf create stack-carrying errors (cockroachdb/errors).
func NewErr(msg string) error { return crdb.New(msg) }

func NewErrf(format string, args ...any) error { return crdb.Newf(format, args...) }
