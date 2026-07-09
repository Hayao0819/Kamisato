package shared

import (
	"os/exec"
	"runtime"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

// OpenBrowser opens url in the user's default browser, detached from ayaka. A
// non-nil error means the caller should fall back to printing the URL.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url) //nolint:gosec // fixed program, url passed as a separate arg (no shell)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url) //nolint:gosec // fixed program, url passed as a separate arg (no shell)
	default:
		cmd = exec.Command("xdg-open", url) //nolint:gosec // fixed program, url passed as a separate arg (no shell)
	}
	if err := cmd.Start(); err != nil {
		return errors.WrapErr(err, "failed to open browser")
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
