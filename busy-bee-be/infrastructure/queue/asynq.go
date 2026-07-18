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
	// dedupeTTL 同一會議在此時間內重複 enqueue 會被去重。
	dedupeTTL = 10 * time.Minute
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
	client *asynq.Client
}

var _ domainmeeting.TaskQueue = (*AsynqQueue)(nil)

func NewAsynq(redisAddr, redisPassword string) *AsynqQueue {
	return &AsynqQueue{
		client: asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr, Password: redisPassword}),
	}
}

func (q *AsynqQueue) Close() error {
	return q.client.Close()
}

func (q *AsynqQueue) EnqueueProcessMeeting(ctx context.Context, meetingID uuid.UUID) error {
	payload, err := json.Marshal(processMeetingPayload{MeetingID: meetingID})
	if err != nil {
		return fmt.Errorf("queue.EnqueueProcessMeeting marshal: %w", err)
	}

	_, err = q.client.EnqueueContext(ctx,
		asynq.NewTask(TaskTypeProcessMeeting, payload),
		asynq.TaskID("meeting:process:"+meetingID.String()), // 同會議去重
		asynq.MaxRetry(processMaxRetry),
		asynq.Timeout(processTimeout),
		asynq.Retention(dedupeTTL),
	)
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil // 已排入，冪等
	}
	if err != nil {
		return fmt.Errorf("queue.EnqueueProcessMeeting: %w", err)
	}
	return nil
}
