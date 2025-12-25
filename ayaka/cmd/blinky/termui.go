package blinkycmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// promptInput shows a simple prompt and reads a single line from stdin.
func promptInput(message string) (string, error) {
	if _, err := fmt.Fprint(os.Stdout, message+" "); err != nil {
		return "", err
	}
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// promptPassword shows a prompt and reads a password without echoing input.
func promptPassword(message string) (string, error) {
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
