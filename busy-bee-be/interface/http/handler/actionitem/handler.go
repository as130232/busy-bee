package actionitem

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	appactionitem "github.com/as130232/busy-bee/busy-bee-be/application/actionitem"
	domainactionitem "github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
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
	Add           *appactionitem.AddUC
	Edit          *appactionitem.EditUC
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

// Add POST /api/v1/meetings/:id/action-items — 手動新增待辦（source=manual，重跑不刪）。
func (h *Handler) Add(c *gin.Context) {
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

	var req struct {
		Description string `json:"description"`
		Assignee    string `json:"assignee"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "body"))
		return
	}

	item, err := h.uc.Add.Execute(c.Request.Context(), userID, meetingID, req.Description, req.Assignee)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"actionItem": toActionItemResponse(item)})
}

// Update PATCH /api/v1/action-items/:id — 部分更新：帶 description 改內容，帶 done 改完成狀態。
func (h *Handler) Update(c *gin.Context) {
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
		Done        *bool   `json:"done"`
		Description *string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "body"))
		return
	}

	var item domainactionitem.ActionItem
	switch {
	case req.Description != nil:
		item, err = h.uc.Edit.Execute(c.Request.Context(), userID, itemID, *req.Description)
	case req.Done != nil:
		item, err = h.uc.Toggle.Execute(c.Request.Context(), userID, itemID, *req.Done)
	default:
		response.Fail(c, apperr.New(errcode.Param))
		return
	}
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"actionItem": toActionItemResponse(item)})
}
