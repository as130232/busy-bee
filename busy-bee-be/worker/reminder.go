package worker

import (
	"context"
	"time"

	appmeeting "github.com/as130232/busy-bee/busy-bee-be/application/meeting"
)

// RunReminderSweep 立即掃一次提醒，之後每 interval 掃描；ctx 取消時結束。
func RunReminderSweep(ctx context.Context, uc *appmeeting.ReminderUC, interval time.Duration) {
	uc.SweepOnce(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			uc.SweepOnce(ctx)
		}
	}
}
