package aurweb

import (
	"slices"
	"strings"
)

// Match reports whether a record satisfies a type=search query under the given
// field, following aurweb's semantics: name/name-desc are case-insensitive
// substring matches; relations are matched by their bare name (the version
// constraint or optdepends description is stripped); a package always provides
// itself.
func Match(p Pkg, by By, arg string) bool {
	arg = strings.ToLower(arg)
	switch by {
	case ByName:
		return strings.Contains(strings.ToLower(p.Name), arg)
	case ByNameDesc, "":
		return strings.Contains(strings.ToLower(p.Name), arg) ||
			strings.Contains(strings.ToLower(p.Description), arg)
	case ByMaintainer:
		return arg == "" || strings.EqualFold(p.Maintainer, arg)
	case ByCoMaintainers:
		return slices.ContainsFunc(p.CoMaintainers, func(c string) bool { return strings.EqualFold(c, arg) })
	case BySubmitter:
		return strings.EqualFold(p.Submitter, arg)
	case ByDepends:
		return hasRelName(p.Depends, arg)
	case ByMakeDepends:
		return hasRelName(p.MakeDepends, arg)
	case ByCheckDepends:
		return hasRelName(p.CheckDepends, arg)
	case ByOptDepends:
		return hasRelName(p.OptDepends, arg)
	case ByProvides:
		return hasRelName(p.Provides, arg) || strings.EqualFold(p.Name, arg)
	case ByConflicts:
		return hasRelName(p.Conflicts, arg)
	case ByReplaces:
		return hasRelName(p.Replaces, arg)
	case ByGroups:
		return slices.ContainsFunc(p.Groups, func(g string) bool { return strings.EqualFold(g, arg) })
	case ByKeywords:
		return slices.ContainsFunc(p.Keywords, func(k string) bool { return strings.EqualFold(k, arg) })
	default:
		return false
	}
}

func hasRelName(rels []string, arg string) bool {
	for _, rel := range rels {
		if strings.EqualFold(relName(rel), arg) {
			return true
		}
	}
	return false
}

// relName strips a version constraint or optdepends description from a relation
// string: "glibc>=2.34" -> "glibc", "foo: a thing" -> "foo".
func relName(s string) string {
	fields := strings.FieldsFunc(s, func(c rune) bool {
		return c == '<' || c == '>' || c == '=' || c == ':' || c == ' '
	})
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
