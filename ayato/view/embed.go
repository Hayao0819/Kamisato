package view

import (
	"embed"
	"html/template"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

func compile() (*template.Template, error) {
	return template.ParseFS(templatesFS, "templates/*")
}

func Set(e *gin.Engine) error {
	t, err := compile()
	if err != nil {
		return errors.Wrap(err, "failed to compile template")
	}
	e.SetHTMLTemplate(t)
	return nil
}
