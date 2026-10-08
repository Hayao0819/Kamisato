package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tmc/langchaingo/llms"

	"github.com/Hayao0819/Kamisato/kayo/audit/llm"
)

// LLMAdvisory sends recipe files to the selected model. The command owns provider
// configuration and credentials; failures never affect trust or exit decisions.
func LLMAdvisory(ctx context.Context, model llms.Model, dir string) (*llm.Advisory, error) {
	pkgbuild, err := os.ReadFile(filepath.Join(dir, "PKGBUILD"))
	if err != nil {
		return nil, fmt.Errorf("skipped (%w)", err)
	}
	// Include every .install scriptlet in a split package recipe.
	var install strings.Builder
	matches, _ := filepath.Glob(filepath.Join(dir, "*.install"))
	for _, match := range matches {
		if contents, err := os.ReadFile(match); err == nil {
			fmt.Fprintf(&install, "--- %s ---\n%s\n", filepath.Base(match), contents)
		}
	}

	advisory, err := llm.Advise(ctx, model, string(pkgbuild), install.String())
	if err != nil {
		return nil, fmt.Errorf("advisory failed (%w)", err)
	}
	return advisory, nil
}
