package middleware

import (
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/response"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// Auth 驗證 Bearer ID token 並檢查 email 白名單，通過後將 Identity 注入 request context。
// 白名單為空時 fail-closed：拒絕所有請求（避免漏設定 = 對全世界開放）。
func Auth(verifier user.TokenVerifier, allowedEmails []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedEmails))
	for _, e := range allowedEmails {
		allowed[strings.ToLower(strings.TrimSpace(e))] = struct{}{}
	}
	if len(allowed) == 0 {
		slog.Warn("auth middleware: ALLOWED_EMAILS is empty, all requests will be rejected (fail-closed)")
	}

	return func(c *gin.Context) {
		token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || token == "" {
			response.Fail(c, apperr.New(errcode.Unauthorized))
			return
		}

		identity, err := verifier.Verify(c.Request.Context(), token)
		if err != nil {
			response.Fail(c, apperr.Wrap(err, errcode.Unauthorized))
			return
		}

		if _, ok := allowed[strings.ToLower(identity.Email)]; !ok {
			response.Fail(c, apperr.New(errcode.Forbidden))
			return
		}

		c.Request = c.Request.WithContext(user.WithIdentity(c.Request.Context(), identity))
		c.Next()
	}
}
