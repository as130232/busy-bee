package meeting

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	appmeeting "github.com/as130232/busy-bee/busy-bee-be/application/meeting"
	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/response"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// Create POST /api/v1/meetings — 建立會議並回傳直傳 signed URL。
func (h *Handler) Create(c *gin.Context) {
	userID, ok := domainuser.IDFrom(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.New(errcode.Unauthorized))
		return
	}

	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "body"))
		return
	}

	out, err := h.uc.Create.Execute(c.Request.Context(), userID, appmeeting.CreateInput{
		Title:       req.Title,
		ContentType: req.ContentType,
		Scenario:    req.Scenario,
		Language:    req.Language,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, toCreateResponse(out))
}

// CompleteUpload POST /api/v1/meetings/:id/complete-upload — 音訊直傳完成後觸發背景處理。
func (h *Handler) CompleteUpload(c *gin.Context) {
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

	m, err := h.uc.CompleteUpload.Execute(c.Request.Context(), userID, meetingID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"meeting": toMeetingResponse(m)})
}
