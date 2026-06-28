package audit

import (
	"bytes"
	"fmt"

	"github.com/Hayao0819/Kamisato/pkg/raiou"
	"mvdan.cc/sh/v3/syntax"
)

// driftCheck flags "manifest confusion": where the committed .SRCINFO contradicts
// the PKGBUILD that runs. Static only — a dynamic pkgver() or $()-built value can't
// be resolved, so it is skipped rather than guessed.
func driftCheck(pkgbuild []byte, si *raiou.SRCINFO) []Finding {
	decl, dynamicVer := declared(pkgbuild)
	var out []Finding

	if pv := first(decl["pkgver"]); pv != "" && si.PkgVer != "" && pv != si.PkgVer && !dynamicVer {
		out = append(out, Finding{
			Code: "DRIFT-VERSION", Severity: SevMedium,
			Title:  "PKGBUILD pkgver differs from .SRCINFO",
			Detail: fmt.Sprintf("PKGBUILD %q vs .SRCINFO %q", pv, si.PkgVer),
		})
	}

	declared := flattenArch(si.Source)
	for _, s := range decl["source"] {
		if !declared[s] {
			out = append(out, Finding{
				Code: "DRIFT-SOURCE", Severity: SevMedium,
				Title:  "PKGBUILD source is not declared in .SRCINFO",
				Detail: s,
			})
			break
		}
	}
	return out
}

// declared extracts literal top-level assignments from the PKGBUILD AST; the bool
// reports whether a pkgver() makes the version dynamic. Non-literals are omitted.
func declared(src []byte) (map[string][]string, bool) {
	f, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(bytes.NewReader(src), "PKGBUILD")
	if err != nil {
		return nil, false
	}

	out := map[string][]string{}
	dynamicVer := false
	syntax.Walk(f, func(n syntax.Node) bool {
		switch x := n.(type) {
		case *syntax.FuncDecl:
			if x.Name != nil && x.Name.Value == "pkgver" {
				dynamicVer = true
			}
		case *syntax.Assign:
			if x.Name == nil {
				return true
			}
			switch {
			case x.Array != nil:
				for _, el := range x.Array.Elems {
					if v := el.Value.Lit(); v != "" {
						out[x.Name.Value] = append(out[x.Name.Value], v)
					}
				}
			case x.Value != nil:
				if v := x.Value.Lit(); v != "" {
					out[x.Name.Value] = append(out[x.Name.Value], v)
				}
			}
		}
		return true
	})
	return out, dynamicVer
}

func flattenArch(a raiou.ArchStrings) map[string]bool {
	set := map[string]bool{}
	for _, vals := range a {
		for _, v := range vals {
			set[v] = true
		}
	}
	return set
}

func first(vals []string) string {
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}
