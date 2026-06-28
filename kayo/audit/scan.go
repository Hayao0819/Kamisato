// Package audit statically inspects a PKGBUILD and its install scriptlets for the
// behaviours real AUR supply-chain attacks have used: piping downloads to a
// shell, fetching dependencies during the build, raw sockets, obfuscation,
// persistence, and insecure sources. Behavioural checks walk the bash AST
// (mvdan.cc/sh) rather than matching lines, so they see across line breaks and
// ignore comments and strings; string-level facts (insecure URLs, weak
// checksums) stay as regex. The PKGBUILD is never executed.
package audit

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

// Severity ranks a finding.
type Severity int

const (
	SevInfo Severity = iota
	SevLow
	SevMedium
	SevHigh
)

func (s Severity) String() string {
	switch s {
	case SevHigh:
		return "high"
	case SevMedium:
		return "medium"
	case SevLow:
		return "low"
	default:
		return "info"
	}
}

// Finding is one triggered check.
type Finding struct {
	Code     string
	Severity Severity
	Title    string
	Detail   string
}

// Report is the result of scanning one package directory.
type Report struct {
	Findings []Finding
}

// Max returns the highest severity among the findings (SevInfo if none).
func (r Report) Max() Severity {
	highest := SevInfo
	for _, f := range r.Findings {
		if f.Severity > highest {
			highest = f.Severity
		}
	}
	return highest
}

type regexCheck struct {
	code  string
	sev   Severity
	title string
	re    *regexp.Regexp
}

// recipeStringChecks are facts the AST adds nothing to.
var recipeStringChecks = []regexCheck{
	{"RAW-SOCKET", SevHigh, "opens a raw socket or interactive shell", regexp.MustCompile(`(?i)(/dev/tcp/|bash\s+-i)`)},
	{"SRC-HTTP", SevLow, "uses an insecure (http) URL", regexp.MustCompile(`(?i)\bhttp://`)},
	{"SRC-IP", SevMedium, "uses a raw-IP URL", regexp.MustCompile(`https?://(\d{1,3}\.){3}\d{1,3}`)},
	{"SRC-SHORTENER", SevMedium, "uses a URL shortener or paste host", regexp.MustCompile(`(?i)https?://(bit\.ly|tinyurl\.com|t\.co|goo\.gl|pastebin\.com|paste\.|transfer\.sh|anonfiles|0x0\.st)`)},
	{"CHECKSUM-SKIP", SevLow, "skips source checksums", regexp.MustCompile(`(?i)sums\s*=\s*\([^)]*\bSKIP\b`)},
	{"CHECKSUM-WEAK", SevLow, "uses a weak checksum algorithm", regexp.MustCompile(`(?i)\b(md5sums|sha1sums)\s*=`)},
}

var installStringChecks = []regexCheck{
	{"INSTALL-PERSIST", SevMedium, "installs persistence (service/timer/cron) as root", regexp.MustCompile(`(?i)(systemctl\s+enable|/etc/systemd/system|crontab|/etc/cron)`)},
}

// Scan inspects the PKGBUILD (and any *.install) in dir.
func Scan(dir string) (Report, error) {
	pkgbuild, err := os.ReadFile(filepath.Join(dir, "PKGBUILD"))
	if err != nil {
		return Report{}, utils.WrapErr(err, "failed to read PKGBUILD")
	}

	var findings []Finding
	findings = append(findings, astChecks(pkgbuild, "PKGBUILD", false)...)
	findings = append(findings, regexChecks(string(pkgbuild), recipeStringChecks)...)
	if si, serr := raiou.ParseSrcinfoFile(filepath.Join(dir, ".SRCINFO")); serr == nil {
		findings = append(findings, driftCheck(pkgbuild, si)...)
	}

	installs, _ := filepath.Glob(filepath.Join(dir, "*.install"))
	for _, p := range installs {
		findings = append(findings, Finding{
			Code: "INSTALL-FILE", Severity: SevLow,
			Title: "ships an install scriptlet that runs as root", Detail: filepath.Base(p),
		})
		if body, rerr := os.ReadFile(p); rerr == nil {
			findings = append(findings, astChecks(body, filepath.Base(p), true)...)
			findings = append(findings, regexChecks(string(body), installStringChecks)...)
		}
	}
	return Report{Findings: findings}, nil
}

func regexChecks(text string, checks []regexCheck) []Finding {
	lines := strings.Split(text, "\n")
	var out []Finding
	for _, c := range checks {
		if loc := firstMatch(lines, c.re); loc != "" {
			out = append(out, Finding{Code: c.code, Severity: c.sev, Title: c.title, Detail: loc})
		}
	}
	return out
}

func firstMatch(lines []string, re *regexp.Regexp) string {
	for i, line := range lines {
		if re.MatchString(line) {
			return "line " + strconv.Itoa(i+1) + ": " + strings.TrimSpace(line)
		}
	}
	return ""
}
