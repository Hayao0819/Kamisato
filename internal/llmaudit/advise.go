// Package llmaudit adds an optional LLM advisory pass over a PKGBUILD: it asks a
// model to triage obfuscation and supply-chain red flags that a static scan
// misses. It is ADVISORY ONLY and never gates an install — an LLM is
// nondeterministic and prompt-injectable, so its verdict is surfaced next to the
// static audit, never in place of it. The build path never calls it; only the
// human-driven audit/trust commands do.
package llmaudit

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/tmc/langchaingo/llms"
)

// Finding is one issue the model flags.
type Finding struct {
	Severity string `json:"severity"` // low | medium | high
	Title    string `json:"title"`
	Detail   string `json:"detail"`
}

// Advisory is the model's triage of a recipe.
type Advisory struct {
	Risk     string    `json:"risk"` // low | medium | high | unknown
	Summary  string    `json:"summary"`
	Findings []Finding `json:"findings"`
}

const systemPrompt = `You are a security reviewer for Arch Linux AUR recipes. The AUR has NO pre-publication review: a PKGBUILD and its .install scriptlets are arbitrary code that runs on the user's machine, and .install hooks (pre/post_install, pre/post_upgrade, pre/post_remove) plus pacman itself run as ROOT. Real AUR attacks have shipped remote-access trojans, infostealers (SSH keys, browser credentials, crypto wallets, cloud/registry tokens), reverse shells, and kernel rootkits this way.

Review the recipe below and flag supply-chain and malicious-code red flags. Look specifically for:

1. Remote code execution at build/install time: piping a download into a shell (curl|bash, wget -O - | sh), downloading a script or binary then chmod +x and running it, or cloning a git repo and executing files from it. (2018: a PKGBUILD ran "curl ... | bash" that installed a systemd timer; 2025: a fake "patches" source entry pointed at a personal repo that ran CHAOS RAT.)

2. Attacker-controlled sources: a source=() entry that is NOT the package's real upstream — pastebins, GitHub gists, raw URLs, IP-literal hosts, URL shorteners, or an unrelated personal repo. Flag when a source host or url= does not match the stated upstream, or when a -bin package fetches from a personal repo instead of an official vendor release.

3. Unexpected dependency execution: invoking npm/pip/go/gem/cargo install of packages a recipe of this nature has no reason to need. (2026 "Atomic Arch": 400+ PKGBUILDs silently ran "npm install" of rogue packages in software unrelated to JavaScript.)

4. Obfuscation: base64/hex/octal decode piped into eval or a shell, encoded blobs, or command strings assembled by concatenation to hide intent.

5. Exfiltration, reverse shells, persistence: bash -i >& /dev/tcp/HOST/PORT, nc -e, socat shells, POSTing system data/keys/wallets to a remote host, or installing persistence (systemd unit/timer, cron, shell-rc edits, eBPF).

6. Code in unexpected places: network calls or shell-outs inside prepare()/build()/package(), and ANY non-trivial logic in a .install scriptlet (it runs as root).

7. Integrity evasion: checksums set to SKIP for a FIXED download URL (SKIP is normal only for VCS/git sources), or pkgver / source-URL / version mismatches.

Judge intent and DRIFT, not style. Do NOT flag ordinary build steps: systemctl daemon-reload, update-desktop-database, or vercmp in a .install; SKIP on a *-git source; or npm/pip in a genuinely JavaScript/Python package are all normal. The core question: does this recipe fetch or run anything that does not belong to building this specific package from its real upstream?`

const responseInstruction = "\n\nRespond with ONLY a JSON object, no prose, no code fences:\n" +
	`{"risk":"low|medium|high","summary":"one sentence","findings":[{"severity":"low|medium|high","title":"short","detail":"what and where"}]}`

// Advise asks the model to triage a PKGBUILD (and optional .install script) and
// returns its advisory. The caller treats the result as advice, not a gate.
func Advise(ctx context.Context, model llms.Model, pkgbuild, install string) (*Advisory, error) {
	if strings.TrimSpace(pkgbuild) == "" {
		return nil, utils.NewErr("llmaudit: empty PKGBUILD")
	}
	out, err := llms.GenerateFromSinglePrompt(ctx, model, buildPrompt(pkgbuild, install),
		llms.WithTemperature(0),
		llms.WithMaxTokens(1024),
	)
	if err != nil {
		return nil, utils.WrapErr(err, "llmaudit: generate")
	}
	return parseAdvisory(out)
}

func buildPrompt(pkgbuild, install string) string {
	var b strings.Builder
	b.WriteString(systemPrompt)
	b.WriteString("\n\n--- PKGBUILD ---\n")
	b.WriteString(pkgbuild)
	if strings.TrimSpace(install) != "" {
		b.WriteString("\n\n--- .install ---\n")
		b.WriteString(install)
	}
	b.WriteString(responseInstruction)
	return b.String()
}

// parseAdvisory extracts the JSON object from the model output, tolerating code
// fences or surrounding prose some models add despite the instruction.
func parseAdvisory(out string) (*Advisory, error) {
	raw := extractJSONObject(out)
	if raw == "" {
		return nil, utils.NewErr("llmaudit: no JSON object in model output")
	}
	var a Advisory
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		return nil, utils.WrapErr(err, "llmaudit: parse advisory JSON")
	}
	a.Risk = normalizeRisk(a.Risk)
	for i := range a.Findings {
		a.Findings[i].Severity = strings.ToLower(strings.TrimSpace(a.Findings[i].Severity))
	}
	return &a, nil
}

// extractJSONObject returns the substring from the first { to the last }, which
// strips ```json fences and any leading/trailing prose.
func extractJSONObject(s string) string {
	i := strings.Index(s, "{")
	j := strings.LastIndex(s, "}")
	if i < 0 || j < i {
		return ""
	}
	return s[i : j+1]
}

func normalizeRisk(r string) string {
	switch strings.ToLower(strings.TrimSpace(r)) {
	case "high":
		return "high"
	case "medium":
		return "medium"
	case "low":
		return "low"
	default:
		return "unknown"
	}
}
