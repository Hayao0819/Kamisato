// Package advisory selects the optional model at the CLI boundary. Construction
// is lazy: help, disabled checks, and failed static audits need no credentials.
package advisory

import (
	"context"
	"fmt"

	"github.com/Hayao0819/Kamisato/kayo/audit/llm"
	kayoconfig "github.com/Hayao0819/Kamisato/kayo/config"
	"github.com/Hayao0819/Kamisato/kayo/service"
)

func Checker(cfg kayoconfig.LLMConfig, force bool) func(context.Context, string) (*llm.Advisory, error) {
	if !cfg.Enabled && !force {
		return nil
	}
	return func(ctx context.Context, dir string) (*llm.Advisory, error) {
		model, err := llm.NewModel(cfg.Provider, cfg.Model, cfg.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("unavailable (%w)", err)
		}
		return service.LLMAdvisory(ctx, model, dir)
	}
}
