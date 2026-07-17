package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/as130232/busy-bee/busy-bee-be/pkg/ctxutil"
)

// RequestLogger 以 slog 結構化記錄每個請求的方法、路徑、狀態與延遲。
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		ctx := c.Request.Context()
		slog.InfoContext(ctx, "http.request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"request_id", ctxutil.RequestID(ctx),
		)
	}
}
