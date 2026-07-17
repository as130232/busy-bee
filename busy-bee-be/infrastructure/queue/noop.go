// Package queue 提供背景任務佇列實作。
// NoopQueue 為 Phase 5 暫時實作，Phase 6 以 Asynq 取代。
package queue

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

type NoopQueue struct{}

var _ domainmeeting.TaskQueue = (*NoopQueue)(nil)

func NewNoop() *NoopQueue { return &NoopQueue{} }

func (q *NoopQueue) EnqueueProcessMeeting(ctx context.Context, meetingID uuid.UUID) error {
	slog.InfoContext(ctx, "queue.noop.enqueue", "meeting_id", meetingID)
	return nil
}
