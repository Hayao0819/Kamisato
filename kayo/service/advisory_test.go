package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tmc/langchaingo/llms"
)

type advisoryModel struct {
	prompt string
	ctx    context.Context
}

type advisoryContextKey struct{}

func (m *advisoryModel) GenerateContent(ctx context.Context, messages []llms.MessageContent, _ ...llms.CallOption) (*llms.ContentResponse, error) {
	m.ctx = ctx
	for _, message := range messages {
		for _, part := range message.Parts {
			if text, ok := part.(llms.TextContent); ok {
				m.prompt += text.Text
			}
		}
	}
	return &llms.ContentResponse{Choices: []*llms.ContentChoice{{Content: `{"risk":"low","summary":"reviewed","findings":[]}`}}}, nil
}

func (*advisoryModel) Call(context.Context, string, ...llms.CallOption) (string, error) {
	panic("advisory must use structured model messages")
}

func TestAdvisoryUsesExplicitModelAndIncludesSplitScriptlets(t *testing.T) {
	dir := t.TempDir()
	for name, contents := range map[string]string{
		"PKGBUILD": "pkgname=example\n", "one.install": "one-scriptlet", "two.install": "two-scriptlet",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	model := &advisoryModel{}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), advisoryContextKey{}, "caller"))
	defer cancel()
	result, err := LLMAdvisory(ctx, model, dir)
	if err != nil || result == nil || result.Summary != "reviewed" {
		t.Fatalf("advisory = %+v, error = %v", result, err)
	}
	for _, want := range []string{"pkgname=example", "one-scriptlet", "two-scriptlet"} {
		if !strings.Contains(model.prompt, want) {
			t.Errorf("recipe did not reach injected model: missing %q", want)
		}
	}
	cancel()
	if model.ctx == nil || model.ctx.Value(advisoryContextKey{}) != "caller" {
		t.Fatal("model operation did not receive the caller's context")
	}
}
