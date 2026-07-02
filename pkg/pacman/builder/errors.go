package builder

import "fmt"

// wrapErr keeps the library layer free of cockroachdb's stack-carrying wrapper;
// a plain %w is all the caller needs to unwrap.
func wrapErr(err error, msg string) error { return fmt.Errorf("%s: %w", msg, err) }
