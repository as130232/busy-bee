package queue

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

func testRedisAddr(t *testing.T) string {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: addr})
	defer client.Close()
	if err := client.Ping(); err != nil {
		t.Skipf("local redis unavailable: %v", err)
	}
	return addr
}

func TestAsynqQueue_EnqueueAndConsume(t *testing.T) {
	addr := testRedisAddr(t)
	meetingID := uuid.New()

	q := NewAsynq(addr, "")
	t.Cleanup(func() { q.Close() })

	received := make(chan uuid.UUID, 16) // 容納前次測試殘留的任務
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: addr},
		asynq.Config{Concurrency: 1, LogLevel: asynq.ErrorLevel},
	)
	mux := asynq.NewServeMux()
	mux.HandleFunc(TaskTypeProcessMeeting, func(_ context.Context, task *asynq.Task) error {
		id, err := ParseProcessMeetingPayload(task.Payload())
		if err != nil {
			t.Errorf("parse payload: %v", err)
			return err
		}
		select {
		case received <- id:
		default:
		}
		return nil
	})
	if err := srv.Start(mux); err != nil {
		t.Fatalf("asynq server start: %v", err)
	}
	t.Cleanup(srv.Shutdown)

	if err := q.EnqueueProcessMeeting(context.Background(), meetingID); err != nil {
		t.Fatalf("EnqueueProcessMeeting() error = %v", err)
	}

	deadline := time.After(5 * time.Second)
	for {
		select {
		case got := <-received:
			if got == meetingID {
				return // 消費到本次任務；殘留的舊任務忽略
			}
		case <-deadline:
			t.Fatal("task not consumed within 5s")
		}
	}
}

func TestAsynqQueue_EnqueueIsIdempotentPerMeeting(t *testing.T) {
	addr := testRedisAddr(t)
	meetingID := uuid.New()

	q := NewAsynq(addr, "")
	t.Cleanup(func() { q.Close() })

	if err := q.EnqueueProcessMeeting(context.Background(), meetingID); err != nil {
		t.Fatalf("first enqueue error = %v", err)
	}
	// 同一會議短時間重複 enqueue 不應報錯（TaskID 去重，冪等）
	if err := q.EnqueueProcessMeeting(context.Background(), meetingID); err != nil {
		t.Fatalf("duplicate enqueue should be silently deduped, got %v", err)
	}
}
