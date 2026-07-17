package middleware

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/as130232/busy-bee/busy-bee-be/interface/http/response"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// Recovery 攔截 panic，回統一 envelope；panic 細節只進 log。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				slog.ErrorContext(c.Request.Context(), "panic recovered",
					"panic", r,
					"path", c.Request.URL.Path,
				)
				response.Fail(c, apperr.New(errcode.Internal))
			}
		}()
		c.Next()
	}
}
