package llmaudit

import (
	"strings"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"
)

// NewModel builds a langchaingo model for the given provider. API keys come from
// the provider's standard environment variable (ANTHROPIC_API_KEY,
// OPENAI_API_KEY), never from config. provider defaults to anthropic; baseURL
// overrides the endpoint (and is the ollama server URL).
func NewModel(provider, model, baseURL string) (llms.Model, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "", "anthropic":
		opts := []anthropic.Option{}
		if model != "" {
			opts = append(opts, anthropic.WithModel(model))
		}
		if baseURL != "" {
			opts = append(opts, anthropic.WithBaseURL(baseURL))
		}
		return anthropic.New(opts...)
	case "openai":
		opts := []openai.Option{}
		if model != "" {
			opts = append(opts, openai.WithModel(model))
		}
		if baseURL != "" {
			opts = append(opts, openai.WithBaseURL(baseURL))
		}
		return openai.New(opts...)
	case "ollama":
		opts := []ollama.Option{}
		if model != "" {
			opts = append(opts, ollama.WithModel(model))
		}
		if baseURL != "" {
			opts = append(opts, ollama.WithServerURL(baseURL))
		}
		return ollama.New(opts...)
	default:
		return nil, utils.NewErrf("llmaudit: unknown provider %q (use anthropic, openai, or ollama)", provider)
	}
}
