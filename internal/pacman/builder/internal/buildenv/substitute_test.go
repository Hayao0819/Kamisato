package buildenv

import (
	"strings"
	"testing"
)

func TestSubstituteBuildPlaceholdersLeavesCommentsIntact(t *testing.T) {
	script := "# uses __EXTRA_REPOS__ and __INSTALL__\nset -e\n__EXTRA_REPOS__\necho ok\n__INSTALL__\n"
	got := SubstituteBuildPlaceholders(script, "cat <<'EOF'\n[repo]\nEOF", "pacman -U x")

	if !strings.Contains(got, "# uses __EXTRA_REPOS__ and __INSTALL__") {
		t.Errorf("comment token was expanded:\n%s", got)
	}
	if strings.Contains(got, "\n__EXTRA_REPOS__\n") || strings.Contains(got, "\n__INSTALL__\n") {
		t.Errorf("standalone placeholder not replaced:\n%s", got)
	}
	if !strings.Contains(got, "[repo]") || !strings.Contains(got, "pacman -U x") {
		t.Errorf("substituted content missing:\n%s", got)
	}
}
