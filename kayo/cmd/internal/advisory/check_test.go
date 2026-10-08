package advisory

import (
	"context"
	"strings"
	"testing"

	kayoconfig "github.com/Hayao0819/Kamisato/kayo/config"
)

func TestCheckerDoesNotConstructDisabledProvider(t *testing.T) {
	cfg := kayoconfig.LLMConfig{Provider: "unknown-provider"}
	if Checker(cfg, false) != nil {
		t.Fatal("disabled advisory should have no model operation")
	}
	check := Checker(cfg, true)
	if check == nil {
		t.Fatal("forced advisory should provide a lazy operation")
	}
	if _, err := check(context.Background(), "unused"); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("provider construction error = %v", err)
	}
}
