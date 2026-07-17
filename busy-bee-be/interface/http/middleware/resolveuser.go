package middleware

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/response"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// ResolveUser 依 Auth 注入的 Identity 查 DB 用戶並注入其 UUID。
// 用戶不存在（未 sync）回 401，前端應重走登入 + /users/sync。
func ResolveUser(repo user.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, ok := user.IdentityFrom(c.Request.Context())
		if !ok {
			response.Fail(c, apperr.New(errcode.Unauthorized))
			return
		}

		u, err := repo.GetByFirebaseUID(c.Request.Context(), identity.UID)
		if err != nil {
			if errors.Is(err, user.ErrNotFound) {
				response.Fail(c, apperr.New(errcode.Unauthorized))
				return
			}
			response.Fail(c, apperr.Wrap(err, errcode.Internal))
			return
		}

		c.Request = c.Request.WithContext(user.WithID(c.Request.Context(), u.ID))
		c.Next()
	}
}
