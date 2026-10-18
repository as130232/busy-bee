package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/as130232/busy-bee/busy-bee-be/interface/http/response"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// BodyLimit 限制請求 body 大小，避免單一請求吃光記憶體。overrides 以路由樣式（c.FullPath()，
// 如 "/api/v1/meetings/:id/contexts"）放寬個別端點。Content-Length 已超過就直接 413；
// 沒帶長度（chunked）時讀到超過上限即中止，handler 讀 body 會拿到 *http.MaxBytesError，
// response.Fail 再轉成 413。
func BodyLimit(limit int64, overrides map[string]int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		n := limit
		if o, ok := overrides[c.FullPath()]; ok {
			n = o
		}
		if c.Request.ContentLength > n {
			response.Fail(c, apperr.New(errcode.RequestTooLarge))
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, n)
		c.Next()
	}
}
