package middleware

import "github.com/gin-gonic/gin"

// SecurityHeaders 設定 API 回應的防禦性安全 header（defense-in-depth）。
// 純 API 後端無 HTML 渲染，故不設 CSP；HSTS 僅在 prod（TLS 由 Cloud Run 終結）啟用。
func SecurityHeaders(prod bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		if prod {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}
