// Package meeting 提供會議相關 HTTP handlers。
package meeting

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	appmeeting "github.com/as130232/busy-bee/busy-bee-be/application/meeting"
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
	Retry          *appmeeting.RetryUC
	Schedule       *appmeeting.ScheduleUC
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

	list, err := h.uc.List.Execute(c.Request.Context(), userID, c.Query("search"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"meetings": toMeetingListResponses(list)})
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
