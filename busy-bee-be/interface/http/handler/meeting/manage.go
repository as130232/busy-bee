package meeting

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/response"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// Rename PATCH /api/v1/meetings/:id — 重新命名會議（任何狀態，本人限定）。
func (h *Handler) Rename(c *gin.Context) {
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
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "body"))
		return
	}

	m, err := h.uc.Manage.Rename(c.Request.Context(), userID, meetingID, req.Title)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"meeting": toMeetingResponse(m)})
}

// UpdateSpeakers PATCH /api/v1/meetings/:id/speakers — 更新講者代號→顯示名（本人限定）。
func (h *Handler) UpdateSpeakers(c *gin.Context) {
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
		SpeakerNames map[string]string `json:"speakerNames"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "body"))
		return
	}

	m, err := h.uc.Manage.UpdateSpeakerNames(c.Request.Context(), userID, meetingID, req.SpeakerNames)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"meeting": toMeetingDetailResponse(m)})
}

// EditSegment PATCH /api/v1/meetings/:id/transcript — 修正單一逐字稿片段文字（本人限定）。
func (h *Handler) EditSegment(c *gin.Context) {
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
		Index int    `json:"index"`
		Text  string `json:"text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "body"))
		return
	}

	m, err := h.uc.EditSegment.Execute(c.Request.Context(), userID, meetingID, req.Index, req.Text)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"meeting": toMeetingDetailResponse(m)})
}

// Delete DELETE /api/v1/meetings/:id — 刪除會議（任何狀態，本人限定；關聯資料連帶刪除）。
func (h *Handler) Delete(c *gin.Context) {
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

	if err := h.uc.Manage.Delete(c.Request.Context(), userID, meetingID); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{})
}

// Retry POST /api/v1/meetings/:id/retry — 失敗會議重新排入處理。
func (h *Handler) Retry(c *gin.Context) {
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

	m, err := h.uc.Retry.Execute(c.Request.Context(), userID, meetingID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"meeting": toMeetingResponse(m)})
}
