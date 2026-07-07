package view

import (
	"embed"
	"html/template"
	"net/http"

	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/gin-gonic/gin"
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

//go:embed static/index.css static/index.js
var staticFS embed.FS

func compile() (*template.Template, error) {
	return template.ParseFS(templatesFS, "templates/*")
}

func Set(e *gin.Engine) error {
	t, err := compile()
	if err != nil {
		return errwrap.WrapErr(err, "failed to compile template")
	}
	e.SetHTMLTemplate(t)
	return nil
}

// SetRepoAssets registers the /repo index page's embedded CSS and JS on g, served
// same-origin so the strict CSP (script-src/style-src 'self') needs no
// 'unsafe-inline', and referenced by absolute path to resolve under any
// /repo/:repo/:arch URL.
func SetRepoAssets(g *gin.RouterGroup) error {
	assets := []struct{ route, file, contentType string }{
		{"/_assets/index.css", "static/index.css", "text/css; charset=utf-8"},
		{"/_assets/index.js", "static/index.js", "text/javascript; charset=utf-8"},
	}
	for _, a := range assets {
		body, err := staticFS.ReadFile(a.file)
		if err != nil {
			return errwrap.WrapErr(err, "failed to read embedded asset "+a.file)
		}
		contentType := a.contentType
		g.GET(a.route, func(c *gin.Context) {
			// Baked into the binary and fixed per build, so immutable caching is safe.
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
			c.Data(http.StatusOK, contentType, body)
		})
	}
	return nil
}
