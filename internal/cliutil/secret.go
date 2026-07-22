package cliutil

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/Hayao0819/Kamisato/internal/errors"
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

// ResolveSecret reads a secret by precedence: the env var, then the file, then
// prompt when non-nil. Empty means no secret.
func ResolveSecret(envVar, file string, prompt func() (string, error)) (string, error) {
	if envVar != "" {
		if s := os.Getenv(envVar); s != "" {
			return s, nil
		}
	}
	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return "", errors.WrapErr(err, "read secret file")
		}
		return strings.TrimRight(string(data), "\n"), nil
	}
	if prompt != nil {
		return prompt()
	}
	return "", nil
}
