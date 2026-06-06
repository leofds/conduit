package server

import "github.com/gin-gonic/gin"

// securityHeaders returns a Gin middleware that sets configured HTTP response headers.
// When headers is nil or empty, no headers are set.
func securityHeaders(headers map[string]string) gin.HandlerFunc {
	if len(headers) == 0 {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		for k, v := range headers {
			c.Header(k, v)
		}
		c.Next()
	}
}
