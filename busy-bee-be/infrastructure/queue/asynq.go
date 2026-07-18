package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

const (
	// TaskTypeProcessMeeting 會議處理任務（STT → 分析 → 完成）。
	TaskTypeProcessMeeting = "meeting:process"

	// processMaxRetry 失敗重試上限；各階段冪等（ADR-009），重試不重複扣費。
	processMaxRetry = 3
	// processTimeout 單次任務時限：90 分鐘音訊的 STT + 生成必須在此內完成。
	processTimeout = 20 * time.Minute

	defaultQueue = "default"
)

type processMeetingPayload struct {
	MeetingID uuid.UUID `json:"meetingId"`
}

func ParseProcessMeetingPayload(b []byte) (uuid.UUID, error) {
	var p processMeetingPayload
	if err := json.Unmarshal(b, &p); err != nil {
		return uuid.Nil, fmt.Errorf("queue.ParseProcessMeetingPayload: %w", err)
	}
	return p.MeetingID, nil
}

// AsynqQueue 以 Asynq 實作 domain/meeting.TaskQueue。
type AsynqQueue struct {
	client    *asynq.Client
	inspector *asynq.Inspector
}

var _ domainmeeting.TaskQueue = (*AsynqQueue)(nil)

// NewAsynq 建立佇列 client。db 為 Redis DB index（production 用 0；測試用獨立 DB 隔離）。
func NewAsynq(redisAddr, redisPassword string, db int) *AsynqQueue {
	opt := asynq.RedisClientOpt{Addr: redisAddr, Password: redisPassword, DB: db}
	return &AsynqQueue{
		client:    asynq.NewClient(opt),
		inspector: asynq.NewInspector(opt),
	}
}

func (q *AsynqQueue) Close() error {
	q.inspector.Close()
	return q.client.Close()
}

// EnqueueProcessMeeting 排入會議處理任務。同會議以 TaskID 去重：
// 執行中的任務視為冪等（不重複排入）；已結束（完成保留/歸檔/失敗）的舊任務刪除後重排，
// 否則手動 retry 會被殘留的舊任務靜默擋下。
func (q *AsynqQueue) EnqueueProcessMeeting(ctx context.Context, meetingID uuid.UUID) error {
	payload, err := json.Marshal(processMeetingPayload{MeetingID: meetingID})
	if err != nil {
		return fmt.Errorf("queue.EnqueueProcessMeeting marshal: %w", err)
	}
	taskID := "meeting:process:" + meetingID.String()
	opts := []asynq.Option{
		asynq.TaskID(taskID),
		asynq.MaxRetry(processMaxRetry),
		asynq.Timeout(processTimeout),
	}
	task := asynq.NewTask(TaskTypeProcessMeeting, payload)

	_, err = q.client.EnqueueContext(ctx, task, opts...)
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		// 舊任務仍佔著 ID：執行中 → DeleteTask 會失敗，視為冪等；否則刪掉重排
		if derr := q.inspector.DeleteTask(defaultQueue, taskID); derr != nil {
			return nil
		}
		if _, err = q.client.EnqueueContext(ctx, task, opts...); err != nil {
			return fmt.Errorf("queue.EnqueueProcessMeeting re-enqueue: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("queue.EnqueueProcessMeeting: %w", err)
	}
	return nil
}
