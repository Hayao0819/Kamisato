package utils

import "github.com/samber/lo"

func Merge[T comparable, Slice ~[]T](slices ...Slice) Slice {
	return lo.Uniq(lo.Flatten(slices))
}
