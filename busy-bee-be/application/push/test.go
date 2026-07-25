package push

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	domainpush "github.com/as130232/busy-bee/busy-bee-be/domain/push"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// TestUC 對指定用戶所有訂閱送出一則測試推播（debug / demo 用，驗證顯示層是否正常）。
// 與 ReminderUC.deliver 相同的 Gone 端點清理語意；不寫任何 reminded 標記。
type TestUC struct {
	repo   domainpush.Repository
	sender domainpush.Sender
}

func NewTestUC(repo domainpush.Repository, sender domainpush.Sender) *TestUC {
	return &TestUC{repo: repo, sender: sender}
}

// SendTest 對 userID 的所有訂閱送測試推播；回傳成功送達數。無訂閱回 NotFound。
func (uc *TestUC) SendTest(ctx context.Context, userID uuid.UUID) (int, error) {
	subs, err := uc.repo.ListByUser(ctx, userID)
	if err != nil {
		return 0, apperr.Wrap(err, errcode.Internal)
	}
	if len(subs) == 0 {
		return 0, apperr.New(errcode.NotFound, "subscription")
	}

	msg := domainpush.Message{
		Title: "測試推播",
		Body:  "看得到這則就代表推播顯示正常 🎉",
		URL:   "/",
	}

	delivered := 0
	for _, sub := range subs {
		switch err := uc.sender.Send(ctx, sub, msg); {
		case err == nil:
			delivered++
		case errors.As(err, &domainpush.ErrSubscriptionGone{}):
			if derr := uc.repo.DeleteByEndpoint(ctx, sub.Endpoint); derr != nil {
				slog.WarnContext(ctx, "push.test.cleanup_failed", "endpoint", sub.Endpoint, "err", derr)
			}
		default:
			slog.WarnContext(ctx, "push.test.send_failed", "err", err)
		}
	}
	return delivered, nil
}
