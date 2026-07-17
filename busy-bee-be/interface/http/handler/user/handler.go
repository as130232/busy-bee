// Package user 提供用戶相關 HTTP handlers。
package user

import (
	"github.com/gin-gonic/gin"

	appuser "github.com/as130232/busy-bee/busy-bee-be/application/user"
	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/response"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

type Handler struct {
	syncUC *appuser.SyncUC
}

func NewHandler(syncUC *appuser.SyncUC) *Handler {
	return &Handler{syncUC: syncUC}
}

// Sync POST /api/v1/users/sync — 登入後同步用戶資料（upsert）。
func (h *Handler) Sync(c *gin.Context) {
	identity, ok := domainuser.IdentityFrom(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.New(errcode.Unauthorized))
		return
	}

	u, err := h.syncUC.Execute(c.Request.Context(), identity)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, toUserResponse(u))
}
