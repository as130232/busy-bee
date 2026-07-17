package meeting

import (
	"context"

	"github.com/google/uuid"
)

// TaskQueue 背景處理任務佇列 port（Asynq 實作於 Phase 6；Phase 5 為 no-op）。
type TaskQueue interface {
	EnqueueProcessMeeting(ctx context.Context, meetingID uuid.UUID) error
}
