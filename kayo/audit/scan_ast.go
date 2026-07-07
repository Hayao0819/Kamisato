package audit

import (
	"bytes"
	"strconv"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

var (
	fetchers   = map[string]bool{"curl": true, "wget": true}
	installers = map[string]bool{"npm": true, "bun": true, "pnpm": true, "yarn": true, "pip": true, "pip3": true, "gem": true}
	shells     = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true}
	sockets    = map[string]bool{"nc": true, "ncat": true, "socat": true}
	installArg = map[string]bool{"install": true, "add": true, "i": true}
)

// parse parses src as bash once; the tree is shared by every AST check, so no file
// is parsed twice per Scan.
func parse(src []byte, name string) (*syntax.File, error) {
	return syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(bytes.NewReader(src), name)
}

// astChecks walks a parsed bash tree. root marks an install scriptlet (runs as
// root), which raises the network/dependency-fetch severities and enables the
// persistence check.
func astChecks(f *syntax.File, root bool) []Finding {
	netCode, netSev, pkgCode := "NET-FETCH", SevMedium, "PKG-INSTALL"
	if root {
		netCode, netSev, pkgCode = "INSTALL-NET", SevHigh, "INSTALL-PKG"
	}

	seen := map[string]bool{}
	var out []Finding
	add := func(code string, sev Severity, title string, pos syntax.Pos) {
		if seen[code] {
			return
		}
		seen[code] = true
		out = append(out, Finding{Code: code, Severity: sev, Title: title, Detail: "line " + strconv.FormatUint(uint64(pos.Line()), 10)})
	}

	syntax.Walk(f, func(n syntax.Node) bool {
		switch x := n.(type) {
		case *syntax.BinaryCmd:
			if (x.Op == syntax.Pipe || x.Op == syntax.PipeAll) && shells[stmtCmdName(x.Y)] {
				add("SHELL-PIPE", SevHigh, "pipes a command into a shell", x.OpPos)
			}
		case *syntax.CallExpr:
			switch name := callName(x); {
			case fetchers[name]:
				add(netCode, netSev, "fetches over the network", x.Pos())
			case installers[name] && hasInstallArg(x):
				add(pkgCode, SevHigh, "fetches packages from another ecosystem", x.Pos())
			case name == "sudo":
				add("SUDO", SevHigh, "invokes sudo", x.Pos())
			case sockets[name]:
				add("RAW-SOCKET", SevHigh, "opens a raw socket", x.Pos())
			case name == "eval" || name == "base64":
				add("OBFUSCATE", SevMedium, "decodes or evaluates dynamic content", x.Pos())
			}
			if root && isPersist(x) {
				add("INSTALL-PERSIST", SevMedium, "installs persistence (service/timer/cron) as root", x.Pos())
			}
		}
		return true
	})
	return out
}

// isPersist reports whether a call installs persistence in an install scriptlet:
// enabling a systemd unit or a crontab, or writing into the system unit/cron
// directories. Reading the AST (not the raw text) means a commented-out line is
// ignored and a quoted path is still inspected.
func isPersist(c *syntax.CallExpr) bool {
	switch callName(c) {
	case "crontab":
		return true
	case "systemctl":
		for _, a := range c.Args[1:] {
			if a.Lit() == "enable" {
				return true
			}
		}
	}
	for _, a := range c.Args {
		t := wordText(a)
		if strings.Contains(t, "/etc/systemd/system") || strings.Contains(t, "/etc/cron") {
			return true
		}
	}
	return false
}

// wordText returns a word's literal text, concatenating literal and quoted
// literal parts and dropping expansions, so a path with an embedded variable
// (e.g. /etc/systemd/system/$name.service) is still matched by substring.
func wordText(w *syntax.Word) string {
	var b strings.Builder
	for _, part := range w.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			b.WriteString(p.Value)
		case *syntax.SglQuoted:
			b.WriteString(p.Value)
		case *syntax.DblQuoted:
			for _, dp := range p.Parts {
				if lit, ok := dp.(*syntax.Lit); ok {
					b.WriteString(lit.Value)
				}
			}
		}
	}
	return b.String()
}

// staticText returns a word's value when it is fully static — a bare, single-, or
// double-quoted literal — and ok=false when any part is an expansion ($var, $(),
// etc.). Quoted literals are captured (unlike Word.Lit), but a value that depends
// on a variable is omitted so a checksum/source comparison never runs on a
// half-resolved string.
func staticText(w *syntax.Word) (string, bool) {
	var b strings.Builder
	for _, part := range w.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			b.WriteString(p.Value)
		case *syntax.SglQuoted:
			b.WriteString(p.Value)
		case *syntax.DblQuoted:
			for _, dp := range p.Parts {
				lit, ok := dp.(*syntax.Lit)
				if !ok {
					return "", false
				}
				b.WriteString(lit.Value)
			}
		default:
			return "", false
		}
	}
	return b.String(), true
}

// callName returns the command name of a CallExpr when it is a plain literal
// ("" for an empty call or a command built from a variable/expansion).
func callName(c *syntax.CallExpr) string {
	if len(c.Args) == 0 {
		return ""
	}
	return c.Args[0].Lit()
}

func stmtCmdName(s *syntax.Stmt) string {
	if s == nil {
		return ""
	}
	if c, ok := s.Cmd.(*syntax.CallExpr); ok {
		return callName(c)
	}
	return ""
}

func hasInstallArg(c *syntax.CallExpr) bool {
	for _, a := range c.Args[1:] {
		if installArg[a.Lit()] {
			return true
		}
	}
	return false
}
