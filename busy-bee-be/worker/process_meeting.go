// Package worker 提供 Asynq task handlers，呼叫 application use cases。
package worker

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"

	appmeeting "github.com/as130232/busy-bee/busy-bee-be/application/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/queue"
)

// NewMux 註冊全部 task handlers。
func NewMux(processUC *appmeeting.ProcessUC) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.HandleFunc(queue.TaskTypeProcessMeeting, processMeetingHandler(processUC))
	return mux
}

func processMeetingHandler(uc *appmeeting.ProcessUC) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		meetingID, err := queue.ParseProcessMeetingPayload(task.Payload())
		if err != nil {
			return err // payload 壞掉重試也沒用，但讓 asynq 歸檔留痕
		}

		if err := uc.Execute(ctx, meetingID); err != nil {
			retried, _ := asynq.GetRetryCount(ctx)
			maxRetry, _ := asynq.GetMaxRetry(ctx)
			slog.ErrorContext(ctx, "worker.process_meeting.attempt_failed",
				"meeting_id", meetingID, "retried", retried, "max_retry", maxRetry, "err", err)

			if retried >= maxRetry {
				// 最後一次重試仍失敗：標 failed（用不受 task timeout 影響的 ctx）
				uc.MarkFailed(context.WithoutCancel(ctx), meetingID, err)
			}
			return err
		}
		return nil
	}
}
