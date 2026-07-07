package middleware

import "github.com/gin-gonic/gin"

// contentSecurityPolicy locks down what the browser may load for ayato's own
// responses. default-src 'none' denies all not re-allowed below;
// frame-ancestors/base-uri/object-src 'none' close the clickjacking, <base>-hijack,
// and plugin vectors. ayato ships no third-party assets, so 'self' suffices.
const contentSecurityPolicy = "default-src 'none'; script-src 'self'; connect-src 'self'; img-src 'self'; style-src 'self'; frame-ancestors 'none'; base-uri 'none'; object-src 'none'"

// SecurityHeaders is applied engine-wide so every route carries the same
// hardening headers.
func (m *Middleware) SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Security-Policy", contentSecurityPolicy)
		c.Next()
	}
}
