// Package ops 提供不對外的維運端點（由 Cloud Scheduler 等內部呼叫者觸發）。
package ops

import (
	"context"
	"crypto/subtle"

	"github.com/gin-gonic/gin"

	"github.com/as130232/busy-bee/busy-bee-be/interface/http/response"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// Sweeper 觸發一輪提醒掃描（由 application/meeting.ReminderUC 實作）。
type Sweeper interface {
	SweepOnce(ctx context.Context)
}

type Handler struct {
	sweeper Sweeper
	secret  string
}

func NewHandler(sweeper Sweeper, secret string) *Handler {
	return &Handler{sweeper: sweeper, secret: secret}
}

// SweepReminders POST /internal/sweep-reminders — 由 Cloud Scheduler 定時觸發提醒掃描。
// 以共享密鑰（X-Sweep-Key header）保護；scale-to-zero 下靠此喚醒 instance 補發提醒（ADR-004）。
func (h *Handler) SweepReminders(c *gin.Context) {
	key := c.GetHeader("X-Sweep-Key")
	// 密鑰未設定或不符即拒絕（fail closed）；constant-time 比較避免 timing 洩漏。
	if h.secret == "" || subtle.ConstantTimeCompare([]byte(key), []byte(h.secret)) != 1 {
		response.Fail(c, apperr.New(errcode.Unauthorized))
		return
	}
	if h.sweeper == nil {
		response.OK(c, gin.H{"swept": false}) // 提醒功能未啟用（無 VAPID）
		return
	}
	h.sweeper.SweepOnce(c.Request.Context())
	response.OK(c, gin.H{"swept": true})
}
