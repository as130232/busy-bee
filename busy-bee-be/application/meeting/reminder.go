package meeting

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	domainactionitem "github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainpush "github.com/as130232/busy-bee/busy-bee-be/domain/push"
)

// ReminderUC 掃描到期的排程會議與到期行動項並推播提醒（ADR-010 掃描式，取代延遲佇列）。
// 全部發送失敗（暫時性錯誤）不標記，下一輪重試；已失效端點（Gone）即刻清除。
type ReminderUC struct {
	meetings    domainmeeting.ReminderRepository
	subs        domainpush.Repository
	sender      domainpush.Sender
	actionItems domainactionitem.ReminderRepository // 可為 nil（未接行動項提醒時）
}

func NewReminderUC(meetings domainmeeting.ReminderRepository, subs domainpush.Repository, sender domainpush.Sender, actionItems domainactionitem.ReminderRepository) *ReminderUC {
	return &ReminderUC{meetings: meetings, subs: subs, sender: sender, actionItems: actionItems}
}

// SweepOnce 執行一輪掃描（會議提醒 + 行動項到期提醒）；錯誤只記 log（由下一輪自然重試）。
func (uc *ReminderUC) SweepOnce(ctx context.Context) {
	uc.sweepMeetings(ctx)
	uc.sweepActionItems(ctx)
}

func (uc *ReminderUC) sweepMeetings(ctx context.Context) {
	due, err := uc.meetings.ListDueReminders(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "reminder.list_failed", "err", err)
		return
	}
	for _, m := range due {
		msg := domainpush.Message{
			Title: "會議提醒",
			Body:  fmt.Sprintf("「%s」將於 %d 分鐘後開始", m.Title, m.RemindBeforeMin),
			URL:   "/?record=1", // 深連結：前端據 record 參數高亮錄音鈕
		}
		if !uc.deliver(ctx, m.UserID, m.ID, msg) {
			continue
		}
		if err := uc.meetings.MarkReminded(ctx, m.ID); err != nil {
			slog.ErrorContext(ctx, "reminder.mark_failed", "meeting_id", m.ID, "err", err)
			continue
		}
		slog.InfoContext(ctx, "reminder.sent", "meeting_id", m.ID)
	}
}

func (uc *ReminderUC) sweepActionItems(ctx context.Context) {
	if uc.actionItems == nil {
		return
	}
	due, err := uc.actionItems.ListDueReminders(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "reminder.action_list_failed", "err", err)
		return
	}
	for _, it := range due {
		msg := domainpush.Message{
			Title: "待辦提醒",
			Body:  fmt.Sprintf("「%s」今天到期", it.Description),
			URL:   "/meetings/" + it.MeetingID.String(), // 深連結：所屬會議詳情
		}
		if !uc.deliver(ctx, it.UserID, it.ID, msg) {
			continue
		}
		if err := uc.actionItems.MarkReminded(ctx, it.ID); err != nil {
			slog.ErrorContext(ctx, "reminder.action_mark_failed", "action_item_id", it.ID, "err", err)
			continue
		}
		slog.InfoContext(ctx, "reminder.action_sent", "action_item_id", it.ID)
	}
}

// deliver 送出訊息給該用戶所有訂閱；回傳是否應標記已提醒。
// 有訂閱但全數暫時性失敗 → 回 false 保留給下一輪重試；Gone 端點即刻清除。
func (uc *ReminderUC) deliver(ctx context.Context, userID, logID uuid.UUID, msg domainpush.Message) bool {
	subs, err := uc.subs.ListByUser(ctx, userID)
	if err != nil {
		slog.ErrorContext(ctx, "reminder.list_subs_failed", "id", logID, "err", err)
		return false
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
			slog.WarnContext(ctx, "reminder.send_failed", "id", logID, "err", err)
		}
	}

	return !(len(subs) > 0 && delivered == 0 && transientFail > 0)
}
