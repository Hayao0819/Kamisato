package oauth

import (
	"os/exec"
	"runtime"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

// OpenSystemBrowser is the default BrowserOpener: it opens rawURL in the user's
// browser, detached from the CLI. A non-nil error means the caller should fall
// back to printing the URL.
func OpenSystemBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL) //nolint:gosec // fixed program, url passed as a separate arg (no shell)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", rawURL) //nolint:gosec // fixed program, url passed as a separate arg (no shell)
	default:
		cmd = exec.Command("xdg-open", rawURL) //nolint:gosec // fixed program, url passed as a separate arg (no shell)
	}
	if err := cmd.Start(); err != nil {
		return errors.WrapErr(err, "failed to open browser")
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
