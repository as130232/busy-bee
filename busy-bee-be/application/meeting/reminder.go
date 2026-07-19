package meeting

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainpush "github.com/as130232/busy-bee/busy-bee-be/domain/push"
)

// ReminderUC 掃描到期的排程會議並推播提醒（ADR-010 掃描式，取代延遲佇列）。
// 全部發送失敗（暫時性錯誤）不標記，下一輪重試；已失效端點（Gone）即刻清除。
type ReminderUC struct {
	meetings domainmeeting.ReminderRepository
	subs     domainpush.Repository
	sender   domainpush.Sender
}

func NewReminderUC(meetings domainmeeting.ReminderRepository, subs domainpush.Repository, sender domainpush.Sender) *ReminderUC {
	return &ReminderUC{meetings: meetings, subs: subs, sender: sender}
}

// SweepOnce 執行一輪掃描；錯誤只記 log（由下一輪自然重試）。
func (uc *ReminderUC) SweepOnce(ctx context.Context) {
	due, err := uc.meetings.ListDueReminders(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "reminder.list_failed", "err", err)
		return
	}

	for _, m := range due {
		uc.remind(ctx, m)
	}
}

func (uc *ReminderUC) remind(ctx context.Context, m domainmeeting.Meeting) {
	subs, err := uc.subs.ListByUser(ctx, m.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "reminder.list_subs_failed", "meeting_id", m.ID, "err", err)
		return
	}

	msg := domainpush.Message{
		Title: "會議提醒",
		Body:  fmt.Sprintf("「%s」將於 %d 分鐘後開始", m.Title, m.RemindBeforeMin),
		URL:   "/?record=1", // 深連結：前端據 record 參數高亮錄音鈕
	}

	delivered, transientFail := 0, 0
	for _, sub := range subs {
		err := uc.sender.Send(ctx, sub, msg)
		switch {
		case err == nil:
			delivered++
		case errors.As(err, &domainpush.ErrSubscriptionGone{}):
			if derr := uc.subs.DeleteByEndpoint(ctx, sub.Endpoint); derr != nil {
				slog.WarnContext(ctx, "reminder.cleanup_failed", "endpoint", sub.Endpoint, "err", derr)
			}
		default:
			transientFail++
			slog.WarnContext(ctx, "reminder.send_failed", "meeting_id", m.ID, "err", err)
		}
	}

	// 有訂閱但全數暫時性失敗 → 保留給下一輪重試
	if len(subs) > 0 && delivered == 0 && transientFail > 0 {
		return
	}

	if err := uc.meetings.MarkReminded(ctx, m.ID); err != nil {
		slog.ErrorContext(ctx, "reminder.mark_failed", "meeting_id", m.ID, "err", err)
		return
	}
	slog.InfoContext(ctx, "reminder.sent", "meeting_id", m.ID, "delivered", delivered)
}
