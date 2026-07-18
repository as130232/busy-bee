package meeting

import (
	"context"

	"github.com/google/uuid"
)

// StatusEvent 會議狀態變更事件（推送給該 user 的前端連線）。
type StatusEvent struct {
	MeetingID    uuid.UUID
	UserID       uuid.UUID
	Status       Status
	ErrorMessage string
}

// StatusNotifier 狀態事件發布 port（單機 in-process WS hub 實作，ADR-010/ADR-002）。
// 通知失敗不影響主流程，實作內部自行處理（log-only）。
type StatusNotifier interface {
	NotifyStatus(ctx context.Context, event StatusEvent)
}
