package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// meetingIndexer 窄介面（*application/search.IndexUC 滿足）。
type meetingIndexer interface {
	Execute(ctx context.Context, meetingID uuid.UUID) error
}

// chunkScanner 窄介面（*db.ChunkRepo 滿足）。
type chunkScanner interface {
	MeetingsWithoutChunks(ctx context.Context) ([]uuid.UUID, error)
}

// backfillOnce 掃一次未索引會議並逐一交給 IndexUC（單筆失敗記 log 不中斷）。
func backfillOnce(ctx context.Context, scanner chunkScanner, index meetingIndexer) {
	ids, err := scanner.MeetingsWithoutChunks(ctx)
	if err != nil {
		slog.WarnContext(ctx, "index.backfill.scan_failed", "err", err)
		return
	}
	for _, id := range ids {
		if err := index.Execute(ctx, id); err != nil {
			slog.WarnContext(ctx, "index.backfill.index_failed", "meeting_id", id, "err", err)
		}
	}
}

// RunIndexBackfill 啟動掃一次，之後每 interval 掃未索引會議補索引，直到 ctx 取消。
func RunIndexBackfill(ctx context.Context, scanner chunkScanner, index meetingIndexer, interval time.Duration) {
	backfillOnce(ctx, scanner, index)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			backfillOnce(ctx, scanner, index)
		}
	}
}
