package aurweb

import (
	"io"
	"strconv"
	"strings"
)

// maxResponseBytes caps how much of an upstream response we buffer, bounding
// memory against a hostile or broken upstream.
const maxResponseBytes = 32 << 20 // 32 MiB

func readAllLimited(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maxResponseBytes))
}

// mergeUnique concatenates lists, keeping the first occurrence of each distinct
// key. Earlier lists win, so callers pass the higher-precedence list first.
func mergeUnique[T any, K comparable](key func(T) K, lists ...[]T) []T {
	seen := map[K]bool{}
	var out []T
	for _, list := range lists {
		for _, v := range list {
			k := key(v)
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, v)
		}
	}
	return out
}

func mergeByName(local, upstream []Pkg) []Pkg {
	return mergeUnique(func(p Pkg) string { return p.Name }, local, upstream)
}

func mergeStrings(local, upstream []string) []string {
	return mergeUnique(func(s string) string { return s }, local, upstream)
}

func dedupeNonEmpty(in []string) []string {
	seen := make(map[string]bool, len(in))
	var out []string
	for _, v := range in {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func parseVersion(seg string) int {
	return atoiSafe(strings.TrimPrefix(seg, "v"))
}

func atoiSafe(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}
