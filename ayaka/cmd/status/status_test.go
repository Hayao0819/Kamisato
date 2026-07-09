package statuscmd

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/fatih/color"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
)

func TestPrintStatusGroups(t *testing.T) {
	color.NoColor = true // deterministic, uncoloured output

	rows := []shared.PkgRow{
		{Package: "chrome", Local: "1.1-1", Remote: "1.0-1", Build: "failed"},
		{Package: "foo", Local: "2.0-1", Remote: "2.0-1", Build: "running"},
		{Package: "bar", Local: "3.0-1", Remote: "-", Build: "-"},
	}

	var buf bytes.Buffer
	printStatus(&buf, rows, false)
	out := buf.String()

	for _, want := range []string{"Build failed", "chrome", "Building", "foo", "Not published", "bar"} {
		if !strings.Contains(out, want) {
			t.Errorf("status output missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "3 package(s) need attention, 0 up to date") {
		t.Errorf("summary unexpected:\n%s", out)
	}
}

func TestPrintStatusAllClean(t *testing.T) {
	color.NoColor = true

	rows := []shared.PkgRow{
		{Package: "foo", Local: "1.0-1", Remote: "1.0-1", Build: "success"},
		{Package: "bar", Local: "2.0-1", Remote: "2.0-1", Build: "success"},
	}

	var buf bytes.Buffer
	printStatus(&buf, rows, false)
	out := buf.String()

	if !strings.Contains(out, "Everything up to date") || !strings.Contains(out, "(2 packages)") {
		t.Errorf("expected clean summary, got:\n%s", out)
	}
}

func TestStatusClassifiers(t *testing.T) {
	if !isFailedStatus("failed") || isFailedStatus("success") {
		t.Error("isFailedStatus misclassified")
	}
	if !isBuildingStatus("queued") || !isBuildingStatus("running") || isBuildingStatus("success") {
		t.Error("isBuildingStatus misclassified")
	}
}

func TestLocalAheadOfRemote(t *testing.T) {
	if localAheadOfRemote("-", "1.0-1") || localAheadOfRemote("1.0-1", "-") {
		t.Error("missing version should never be ahead")
	}
	if _, err := exec.LookPath("vercmp"); err != nil {
		t.Skip("vercmp not available")
	}
	if !localAheadOfRemote("1.1-1", "1.0-1") {
		t.Error("1.1-1 should be ahead of 1.0-1")
	}
	if localAheadOfRemote("1.0-1", "1.0-1") {
		t.Error("equal versions are not ahead")
	}
}
