// Package audit statically inspects a PKGBUILD and its install scriptlets for
// behaviours seen in AUR supply-chain attacks. Each file is parsed once as a bash
// AST (mvdan.cc/sh); behavioural and integrity checks walk that tree instead of
// matching lines, so they see across line breaks and ignore comments and strings.
// Only insecure/suspicious source URLs stay as regex string-facts. The PKGBUILD is
// never executed.
package audit

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

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

type Finding struct {
	Code     string
	Severity Severity
	Title    string
	Detail   string
}

type Report struct {
	Findings []Finding
}

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

// recipeStringChecks are string facts the AST adds nothing to: raw sockets and
// insecure or suspicious source URLs.
var recipeStringChecks = []regexCheck{
	{"RAW-SOCKET", SevHigh, "opens a raw socket or interactive shell", regexp.MustCompile(`(?i)(/dev/tcp/|bash\s+-i)`)},
	{"SRC-HTTP", SevLow, "uses an insecure (http) URL", regexp.MustCompile(`(?i)\bhttp://`)},
	{"SRC-IP", SevMedium, "uses a raw-IP URL", regexp.MustCompile(`https?://(\d{1,3}\.){3}\d{1,3}`)},
	{"SRC-SHORTENER", SevMedium, "uses a URL shortener or paste host", regexp.MustCompile(`(?i)https?://(bit\.ly|tinyurl\.com|t\.co|goo\.gl|pastebin\.com|paste\.|transfer\.sh|anonfiles|0x0\.st)`)},
}

func Scan(dir string) (Report, error) {
	pkgbuild, err := os.ReadFile(filepath.Join(dir, "PKGBUILD"))
	if err != nil {
		return Report{}, errwrap.WrapErr(err, "failed to read PKGBUILD")
	}

	var findings []Finding
	if f, perr := parse(pkgbuild, "PKGBUILD"); perr != nil {
		findings = append(findings, parseFinding(perr))
	} else {
		findings = append(findings, astChecks(f, false)...)
		decl, dynamicVer := declaredFrom(f)
		findings = append(findings, checksumFindings(decl)...)
		if si, serr := raiou.ParseSrcinfoFile(filepath.Join(dir, ".SRCINFO")); serr == nil {
			findings = append(findings, driftCheck(decl, dynamicVer, si)...)
		}
	}
	findings = append(findings, regexChecks(string(pkgbuild), recipeStringChecks)...)

	installs, _ := filepath.Glob(filepath.Join(dir, "*.install"))
	for _, p := range installs {
		findings = append(findings, Finding{
			Code: "INSTALL-FILE", Severity: SevLow,
			Title: "ships an install scriptlet that runs as root", Detail: filepath.Base(p),
		})
		if body, rerr := os.ReadFile(p); rerr == nil {
			if f, perr := parse(body, filepath.Base(p)); perr != nil {
				findings = append(findings, parseFinding(perr))
			} else {
				findings = append(findings, astChecks(f, true)...)
			}
		}
	}
	return Report{Findings: findings}, nil
}

// checksumFindings flags a *sums array holding SKIP (an integrity opt-out only
// normal for VCS sources) or a weak md5/sha1 checksum. Reading the AST catches a
// multi-line array and ignores a commented-out line, which a per-line regex can't.
func checksumFindings(decl map[string][]string) []Finding {
	var out []Finding

	skipVar := ""
	for name, vals := range decl {
		if !strings.HasSuffix(name, "sums") {
			continue
		}
		for _, v := range vals {
			if v == "SKIP" {
				skipVar = name
			}
		}
	}
	if skipVar != "" {
		out = append(out, Finding{Code: "CHECKSUM-SKIP", Severity: SevLow, Title: "skips source checksums", Detail: skipVar})
	}

	switch {
	case len(decl["md5sums"]) > 0:
		out = append(out, Finding{Code: "CHECKSUM-WEAK", Severity: SevLow, Title: "uses a weak checksum algorithm", Detail: "md5sums"})
	case len(decl["sha1sums"]) > 0:
		out = append(out, Finding{Code: "CHECKSUM-WEAK", Severity: SevLow, Title: "uses a weak checksum algorithm", Detail: "sha1sums"})
	}
	return out
}

func parseFinding(err error) Finding {
	return Finding{Code: "PARSE", Severity: SevLow, Title: "did not parse as bash", Detail: strings.TrimSpace(err.Error())}
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
