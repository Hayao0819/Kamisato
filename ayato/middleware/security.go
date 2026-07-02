package middleware

import "github.com/gin-gonic/gin"

// contentSecurityPolicy locks down what the browser may load for ayato's own
// responses (the JSON API and the plain /repo index). default-src 'none' denies
// everything not re-allowed below; frame-ancestors/base-uri/object-src 'none'
// close the clickjacking, <base>-hijack, and plugin vectors. ayato ships no
// third-party assets, so 'self' is the only source any directive needs.
const contentSecurityPolicy = "default-src 'none'; script-src 'self'; connect-src 'self'; img-src 'self'; style-src 'self'; frame-ancestors 'none'; base-uri 'none'; object-src 'none'"

// SecurityHeaders is applied engine-wide so every route — the API, the /repo
// index, and the AUR NoRoute fallback — carries the same hardening headers.
func (m *Middleware) SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Security-Policy", contentSecurityPolicy)
		c.Next()
	}
}
