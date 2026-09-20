package meeting

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/response"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

type scheduleRequest struct {
	Title           string `json:"title"`
	Scenario        string `json:"scenario"`    // meeting/casual；省略回退 meeting
	Language        string `json:"language"`    // zh-TW/en-US/auto；省略回退 zh-TW
	ScheduledAt     string `json:"scheduledAt"` // RFC3339
	RemindBeforeMin int    `json:"remindBeforeMin"`
}

func (r scheduleRequest) toParams() (domainmeeting.ScheduleParams, error) {
	at, err := time.Parse(time.RFC3339, r.ScheduledAt)
	if err != nil {
		return domainmeeting.ScheduleParams{}, err
	}
	return domainmeeting.ScheduleParams{
		Title:           r.Title,
		Scenario:        domainmeeting.ParseScenario(r.Scenario),
		Language:        domainmeeting.ParseLanguage(r.Language),
		ScheduledAt:     at,
		RemindBeforeMin: r.RemindBeforeMin,
	}, nil
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
