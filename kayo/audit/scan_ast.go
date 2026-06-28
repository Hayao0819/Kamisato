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

// astChecks walks the bash AST. root marks an install scriptlet (runs as root),
// which raises the network/dependency-fetch severities.
func astChecks(src []byte, name string, root bool) []Finding {
	f, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(bytes.NewReader(src), name)
	if err != nil {
		return []Finding{{Code: "PARSE", Severity: SevLow, Title: "did not parse as bash", Detail: strings.TrimSpace(err.Error())}}
	}

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
		}
		return true
	})
	return out
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
