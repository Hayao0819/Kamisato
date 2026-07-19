// Package shellutil contains shell rendering shared by sandbox implementations.
package shellutil

import "strings"

func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
