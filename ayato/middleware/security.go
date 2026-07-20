package middleware

import (
	"github.com/gin-contrib/secure"
	"github.com/gin-gonic/gin"
)

// default-src denies resources not explicitly allowed; the remaining
// directives close browser embedding and base/plugin injection paths.
const contentSecurityPolicy = "default-src 'none'; script-src 'self'; connect-src 'self'; img-src 'self'; style-src 'self'; frame-ancestors 'none'; base-uri 'none'; object-src 'none'"

// SecurityHeaders centralizes the engine-wide browser hardening policy.
func (m *Middleware) SecurityHeaders() gin.HandlerFunc {
	return secure.New(secure.Config{ContentSecurityPolicy: contentSecurityPolicy})
}
