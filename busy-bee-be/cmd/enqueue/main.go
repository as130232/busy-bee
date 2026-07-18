// enqueue 手動將指定會議排入處理佇列（維運/重跑用）。
// 用法：go run ./cmd/enqueue <meeting-id>
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/google/uuid"

	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/config"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/queue"
)

func main() {
	if len(os.Args) < 2 {
		slog.Error("usage: enqueue <meeting-id>")
		os.Exit(1)
	}
	id, err := uuid.Parse(os.Args[1])
	if err != nil {
		slog.Error("invalid meeting id", "err", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}

	q := queue.NewAsynq(cfg.Redis.Addr, cfg.Redis.Password)
	defer q.Close()

	if err := q.EnqueueProcessMeeting(context.Background(), id); err != nil {
		slog.Error("enqueue failed", "err", err)
		os.Exit(1)
	}
	slog.Info("enqueued", "meeting_id", id)
}
