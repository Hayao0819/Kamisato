package cmd

import (
	"os/exec"
	"runtime"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// openBrowser opens url in the user's default browser, detached from ayaka. A
// non-nil error means the caller should fall back to printing the URL.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return utils.WrapErr(err, "failed to open browser")
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
