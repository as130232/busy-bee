package actionitem

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	appactionitem "github.com/as130232/busy-bee/busy-bee-be/application/actionitem"
	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/response"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// HandlerUCs Handler 依賴的 use cases。
type HandlerUCs struct {
	ListByMeeting *appactionitem.ListByMeetingUC
	ListPending   *appactionitem.ListPendingUC
	Toggle        *appactionitem.ToggleUC
}

type Handler struct {
	uc HandlerUCs
}

func NewHandler(uc HandlerUCs) *Handler {
	return &Handler{uc: uc}
}

// ListByMeeting GET /api/v1/meetings/:id/action-items — 取回某會議的行動項。
func (h *Handler) ListByMeeting(c *gin.Context) {
	userID, ok := domainuser.IDFrom(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.New(errcode.Unauthorized))
		return
	}
	meetingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "id"))
		return
	}

	list, err := h.uc.ListByMeeting.Execute(c.Request.Context(), userID, meetingID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"actionItems": toActionItemResponses(list)})
}

// ListPending GET /api/v1/action-items — 跨會議未完成行動項。
func (h *Handler) ListPending(c *gin.Context) {
	userID, ok := domainuser.IDFrom(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.New(errcode.Unauthorized))
		return
	}

	list, err := h.uc.ListPending.Execute(c.Request.Context(), userID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"actionItems": toPendingActionItemResponses(list)})
}

// Toggle PATCH /api/v1/action-items/:id — 標記完成 / 取消完成。
func (h *Handler) Toggle(c *gin.Context) {
	userID, ok := domainuser.IDFrom(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.New(errcode.Unauthorized))
		return
	}
	itemID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "id"))
		return
	}

	var req struct {
		Done bool `json:"done"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "body"))
		return
	}

	item, err := h.uc.Toggle.Execute(c.Request.Context(), userID, itemID, req.Done)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"actionItem": toActionItemResponse(item)})
}
