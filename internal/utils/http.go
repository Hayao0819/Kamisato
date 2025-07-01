package utils

import (
	"net/http"

	"github.com/samber/lo"
)

func MultipartFormNames(r *http.Request) []string {
	if r.MultipartForm == nil {
		return nil
	}
	return lo.Keys(r.MultipartForm.File)
}
