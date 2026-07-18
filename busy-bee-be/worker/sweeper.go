// Package worker 提供背景處理的組裝件：復原掃描（Sweeper）。
// 記憶體佇列（ADR-010）在重啟時遺失任務；Sweeper 以 DB 為真相源，
// 啟動時與定期將未完成的會議重新入列，佇列的 in-flight 去重避免重複執行。
package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

// UnfinishedLister 供掃描的最小依賴（domain/meeting.Repository 的子集）。
type UnfinishedLister interface {
	ListUnfinishedIDs(ctx context.Context) ([]uuid.UUID, error)
}

type Sweeper struct {
	lister UnfinishedLister
	queue  domainmeeting.TaskQueue
}

func NewSweeper(lister UnfinishedLister, queue domainmeeting.TaskQueue) *Sweeper {
	return &Sweeper{lister: lister, queue: queue}
}

// Run 立即掃一次，之後每 interval 掃描；ctx 取消時結束。
func (s *Sweeper) Run(ctx context.Context, interval time.Duration) {
	s.sweep(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweep(ctx)
		}
	}
}

func (s *Sweeper) sweep(ctx context.Context) {
	ids, err := s.lister.ListUnfinishedIDs(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "worker.sweeper.list_failed", "err", err)
		return
	}
	for _, id := range ids {
		if err := s.queue.EnqueueProcessMeeting(ctx, id); err != nil {
			slog.WarnContext(ctx, "worker.sweeper.enqueue_failed", "meeting_id", id, "err", err)
		}
	}
	if len(ids) > 0 {
		slog.InfoContext(ctx, "worker.sweeper.swept", "count", len(ids))
	}
}
