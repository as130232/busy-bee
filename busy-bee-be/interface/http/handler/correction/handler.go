// Package correction 提供 AI 錄音重新校正（F-CORRECT）HTTP handlers。
package correction

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	domaincorrection "github.com/as130232/busy-bee/busy-bee-be/domain/correction"
	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/response"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// stateUC 三個端點共用的 use case 形狀（GetUC / StartUC / RevertUC）。
type stateUC interface {
	Execute(ctx context.Context, userID, meetingID uuid.UUID) (domaincorrection.State, error)
}

type Handler struct {
	get, start, revert stateUC
}

func NewHandler(get, start, revert stateUC) *Handler {
	return &Handler{get: get, start: start, revert: revert}
}

// stateResponse 校正狀態；status 為空字串代表尚未校正過。
type stateResponse struct {
	Status       string `json:"status"`
	AttemptsUsed int    `json:"attemptsUsed"`
	AttemptsMax  int    `json:"attemptsMax"`
	CanRevert    bool   `json:"canRevert"`
	ErrorMessage string `json:"errorMessage"`
}

// Get GET /api/v1/meetings/:id/correction — 最新一次校正狀態。
func (h *Handler) Get(c *gin.Context) { h.run(c, h.get) }

// Start POST /api/v1/meetings/:id/correction — 建立 AI 校正並排入背景處理。
func (h *Handler) Start(c *gin.Context) { h.run(c, h.start) }

// Revert POST /api/v1/meetings/:id/correction/revert — 整份還原最新一次已完成的校正。
func (h *Handler) Revert(c *gin.Context) { h.run(c, h.revert) }

func (h *Handler) run(c *gin.Context, uc stateUC) {
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
	st, err := uc.Execute(c.Request.Context(), userID, meetingID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"correction": stateResponse{
		Status: string(st.Status), AttemptsUsed: st.AttemptsUsed, AttemptsMax: domaincorrection.MaxAttempts,
		CanRevert: st.CanRevert, ErrorMessage: st.ErrorMessage,
	}})
}
