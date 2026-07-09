// Dependency-spec parsing: a package name with an optional version constraint,
// e.g. "glibc>=2.38", and whether a concrete version satisfies one. The same
// syntax appears in depends/makedepends and in provides entries.
package depend

import (
	"fmt"
	"strings"

	"github.com/Hayao0819/Kamisato/pkg/pacman/alpm"
)

type Op string

const (
	OpAny Op = ""   // no constraint: any version satisfies
	OpEQ  Op = "="  // exact
	OpGE  Op = ">=" // at least
	OpLE  Op = "<=" // at most
	OpGT  Op = ">"  // strictly greater
	OpLT  Op = "<"  // strictly less
)

// Constraint is a parsed dependency spec.
type Constraint struct {
	Name string
	Op   Op
	Ver  string
}

// twoChar operators must be tried before their one-char prefixes (">=" before ">").
var ops = []Op{OpGE, OpLE, OpGT, OpLT, OpEQ}

// Parse splits a dep spec into name and optional version constraint;
// epoch/pkgrel are preserved (e.g. "1:2.3-4") for alpm's vercmp.
func Parse(spec string) Constraint {
	s := strings.TrimSpace(spec)
	for _, op := range ops {
		if i := strings.Index(s, string(op)); i > 0 {
			return Constraint{
				Name: strings.TrimSpace(s[:i]),
				Op:   op,
				Ver:  strings.TrimSpace(s[i+len(op):]),
			}
		}
	}
	return Constraint{Name: s}
}

// Satisfies reports whether version meets the constraint, comparing with alpm's
// vercmp. OpAny is always satisfied.
func (c Constraint) Satisfies(version string) (bool, error) {
	if c.Op == OpAny {
		return true, nil
	}
	cmp, err := alpm.VerCmp(version, c.Ver)
	if err != nil {
		return false, err
	}
	switch c.Op {
	case OpEQ:
		return cmp == 0, nil
	case OpGE:
		return cmp >= 0, nil
	case OpLE:
		return cmp <= 0, nil
	case OpGT:
		return cmp > 0, nil
	case OpLT:
		return cmp < 0, nil
	}
	return false, fmt.Errorf("dep: unknown operator %q", c.Op)
}
