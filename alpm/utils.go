package alpm

import goalpm "github.com/Jguer/go-alpm/v2"

func VerCmp(v1, v2 string) int {
	return goalpm.VerCmp(v1, v2)
}
