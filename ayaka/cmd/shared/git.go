package shared

import (
	"github.com/Hayao0819/Kamisato/internal/gitcmd"
)

func GitRootDir(dir string) (string, error) {
	return gitcmd.RepoRoot(dir)
}
