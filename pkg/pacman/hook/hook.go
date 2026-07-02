// Package hook installs and removes libalpm (pacman) hook files. A tool
// embeds its own .hook template carrying the @EXEC@ placeholder and calls
// Install with the concrete command pacman should run; Uninstall removes it.
// Centralizing the placement keeps the path, permissions, and root-needed error
// wording identical across kayo (verify-on-install) and ayaka (upload-on-install).
package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExecPlaceholder is the token in a template that Install swaps for the Exec line.
const ExecPlaceholder = "@EXEC@"

// ValidateExecArg rejects a value that would re-tokenize on a hook's Exec line.
// pacman word-splits Exec on whitespace (no shell), so a baked value containing
// whitespace or quotes injects extra argv flags into the hooked command rather
// than passing through as one argument. Callers validate user-supplied values
// before baking them.
func ValidateExecArg(name, v string) error {
	if strings.ContainsAny(v, " \t\r\n\"'\\") {
		return fmt.Errorf("%s contains whitespace or quotes and cannot be baked into a hook's Exec line: %q", name, v)
	}
	return nil
}

// Install renders template (replacing @EXEC@ with exec) and writes it as
// dir/fileName, returning the written path.
func Install(dir, fileName, template, exec string) (string, error) {
	content := strings.ReplaceAll(template, ExecPlaceholder, exec)
	path := filepath.Join(dir, fileName)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // pacman hooks must be world-readable (0644 matches the libalpm hook-dir convention)
		return "", fmt.Errorf("failed to write pacman hook (root needed for %s?): %w", dir, err)
	}
	return path, nil
}

// Uninstall removes dir/fileName. A missing file is not an error.
func Uninstall(dir, fileName string) (string, error) {
	path := filepath.Join(dir, fileName)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return path, nil
}
