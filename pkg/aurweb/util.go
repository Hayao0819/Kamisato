package aurweb

import (
	"io"
	"slices"
	"strconv"
	"strings"
)

// maxResponseBytes caps how much of an upstream response we buffer, bounding
// memory against a hostile or broken upstream.
const maxResponseBytes = 32 << 20 // 32 MiB

func readAllLimited(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maxResponseBytes))
}

// DedupeBy keeps the first occurrence of each distinct key, preserving order.
// Merging callers concatenate the higher-precedence items first, so the survivor
// of a name collision is the one that ranks highest.
func DedupeBy[T any, K comparable](items []T, key func(T) K) []T {
	seen := make(map[K]bool, len(items))
	var out []T
	for _, v := range items {
		k := key(v)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, v)
	}
	return out
}

func mergeByName(local, upstream []Pkg) []Pkg {
	return DedupeBy(slices.Concat(local, upstream), func(p Pkg) string { return p.Name })
}

func mergeStrings(local, upstream []string) []string {
	return DedupeBy(slices.Concat(local, upstream), func(s string) string { return s })
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
