package aurweb

import (
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/samber/lo"
)

// maxResponseBytes caps how much of an upstream response we buffer, bounding
// memory against a hostile or broken upstream.
const maxResponseBytes = 32 << 20 // 32 MiB

func readAllLimited(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maxResponseBytes))
}

// DedupeBy keeps the first occurrence of each distinct key in input order.
func DedupeBy[T any, K comparable](items []T, key func(T) K) []T {
	return lo.UniqBy(items, key)
}

func mergeByName(local, upstream []Pkg) []Pkg {
	return lo.UniqBy(slices.Concat(local, upstream), func(p Pkg) string { return p.Name })
}

func mergeStrings(local, upstream []string) []string {
	return lo.Uniq(slices.Concat(local, upstream))
}

func dedupeNonEmpty(in []string) []string {
	return lo.Uniq(lo.Compact(in))
}

func sortedNonEmpty(lists ...[]string) []string {
	out := dedupeNonEmpty(slices.Concat(lists...))
	slices.Sort(out)
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
