// Package errors adds stack-carrying errors at application and adapter
// boundaries. Pure library code may use the standard library's errors package;
// both forms support the same errors.Is/errors.As chains. The stack backend
// stays here so callers do not depend directly on cockroachdb/errors.
package errors

import (
	stderrors "errors"

	crdb "github.com/cockroachdb/errors"
	"github.com/cockroachdb/errors/errbase"
)

// New, Is, As, Join, and ErrUnsupported mirror the standard library's errors
// package for callers that also need the stack helpers in this package.
func New(text string) error         { return stderrors.New(text) }
func Is(err, target error) bool     { return stderrors.Is(err, target) }
func As(err error, target any) bool { return stderrors.As(err, target) }
func Join(errs ...error) error      { return stderrors.Join(errs...) }

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
