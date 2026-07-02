package shared

import (
	"context"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/gitcmd"
)

func GitRootDir(dir string) (string, error) {
	out, err := gitcmd.Output(context.Background(), dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
