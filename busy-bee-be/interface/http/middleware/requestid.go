package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"

	"github.com/as130232/busy-bee/busy-bee-be/pkg/ctxutil"
)

const HeaderRequestID = "X-Request-Id"

// RequestID 為每個請求產生 ID，注入 request context 與 response header。
// 上游（LB / gateway）已帶 X-Request-Id 時沿用。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if id == "" {
			id = newID()
		}
		c.Request = c.Request.WithContext(ctxutil.WithRequestID(c.Request.Context(), id))
		c.Writer.Header().Set(HeaderRequestID, id)
		c.Next()
	}
}

func newID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(b)
}
