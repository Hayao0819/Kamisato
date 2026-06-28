package shared

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/term"
)

// PromptPassword shows a prompt and reads a password without echoing input.
func PromptPassword(message string) (string, error) {
	if _, err := fmt.Fprint(os.Stdout, message+" "); err != nil {
		return "", err
	}
	bytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stdout)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
