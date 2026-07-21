// Package meeting 提供會議相關 HTTP handlers。
package meeting

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	appmeeting "github.com/as130232/busy-bee/busy-bee-be/application/meeting"
	appsearch "github.com/as130232/busy-bee/busy-bee-be/application/search"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/response"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// HandlerUCs Handler 依賴的 use cases。
type HandlerUCs struct {
	Create         *appmeeting.CreateUC
	CompleteUpload *appmeeting.CompleteUploadUC
	ListArtifacts  *appmeeting.ListArtifactsUC
	List           *appmeeting.ListUC
	Get            *appmeeting.GetUC
	AudioURL       *appmeeting.AudioURLUC
	Retry          *appmeeting.RetryUC
	Schedule       *appmeeting.ScheduleUC
	Manage         *appmeeting.ManageUC
	EditSegment    *appmeeting.EditSegmentUC
	Search         *appsearch.SearchUC // 選填；nil 時 List 維持純字面
}

type Handler struct {
	uc HandlerUCs
}

func NewHandler(uc HandlerUCs) *Handler {
	return &Handler{uc: uc}
}

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

// ListArtifacts GET /api/v1/meetings/:id/artifacts — 取回生成文件。
func (h *Handler) ListArtifacts(c *gin.Context) {
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

	list, err := h.uc.ListArtifacts.Execute(c.Request.Context(), userID, meetingID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"artifacts": toArtifactResponses(list)})
}

// List GET /api/v1/meetings?search= — 本人會議列表（新→舊）。
func (h *Handler) List(c *gin.Context) {
	userID, ok := domainuser.IDFrom(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.New(errcode.Unauthorized))
		return
	}

	query := strings.TrimSpace(c.Query("search"))
	// search 非空且已注入 SearchUC → 走 hybrid（字面 + 語意）；否則維持純字面列表
	if query == "" || h.uc.Search == nil {
		list, err := h.uc.List.Execute(c.Request.Context(), userID, query)
		if err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, gin.H{"meetings": toMeetingListResponses(list)})
		return
	}

	meetings, hits, err := h.uc.Search.Execute(c.Request.Context(), userID, query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"meetings": toSearchResponses(meetings, hits)})
}

// Get GET /api/v1/meetings/:id — 會議詳情（含 transcript）。
func (h *Handler) Get(c *gin.Context) {
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

	m, err := h.uc.Get.Execute(c.Request.Context(), userID, meetingID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"meeting": toMeetingDetailResponse(m)})
}

type scheduleRequest struct {
	Title           string `json:"title"`
	ScheduledAt     string `json:"scheduledAt"` // RFC3339
	RemindBeforeMin int    `json:"remindBeforeMin"`
}

func (r scheduleRequest) toParams() (domainmeeting.ScheduleParams, error) {
	at, err := time.Parse(time.RFC3339, r.ScheduledAt)
	if err != nil {
		return domainmeeting.ScheduleParams{}, err
	}
	return domainmeeting.ScheduleParams{Title: r.Title, ScheduledAt: at, RemindBeforeMin: r.RemindBeforeMin}, nil
}

// CreateScheduled POST /api/v1/meetings/scheduled — 建立未來會議（提醒用）。
func (h *Handler) CreateScheduled(c *gin.Context) {
	userID, ok := domainuser.IDFrom(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.New(errcode.Unauthorized))
		return
	}
	var req scheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "body"))
		return
	}
	params, err := req.toParams()
	if err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "scheduledAt"))
		return
	}

	m, err := h.uc.Schedule.Create(c.Request.Context(), userID, params)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"meeting": toMeetingResponse(m)})
}

// UpdateSchedule PATCH /api/v1/meetings/:id/schedule — 修改排程（清除已提醒標記）。
func (h *Handler) UpdateSchedule(c *gin.Context) {
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
	var req scheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "body"))
		return
	}
	params, err := req.toParams()
	if err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "scheduledAt"))
		return
	}

	m, err := h.uc.Schedule.Update(c.Request.Context(), userID, meetingID, params)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"meeting": toMeetingResponse(m)})
}

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

// AudioURL GET /api/v1/meetings/:id/audio-url — 取得本人會議音檔的限時播放 URL。
func (h *Handler) AudioURL(c *gin.Context) {
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
	url, err := h.uc.AudioURL.Execute(c.Request.Context(), userID, meetingID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"url": url})
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
