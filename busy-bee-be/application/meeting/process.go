package meeting

import (
	"context"
	"fmt"
	"log/slog"
	"path"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

// ProcessUC 會議處理管線：pending → transcribing（STT）→ analyzing → completed。
// 各階段冪等（ADR-009）：已有產物的階段直接跳過，retry 不重複呼叫外部 API。
// 失敗時回傳錯誤交由 Asynq retry；標記 failed 是 worker 在最後一次重試後的決定（MarkFailed）。
type ProcessUC struct {
	repo     domainmeeting.Repository
	storage  domainmeeting.AudioStorage
	stt      domainmeeting.STTClient
	notifier domainmeeting.StatusNotifier
}

func NewProcessUC(repo domainmeeting.Repository, storage domainmeeting.AudioStorage, stt domainmeeting.STTClient, notifier domainmeeting.StatusNotifier) *ProcessUC {
	return &ProcessUC{repo: repo, storage: storage, stt: stt, notifier: notifier}
}

func (uc *ProcessUC) notify(ctx context.Context, m domainmeeting.Meeting) {
	uc.notifier.NotifyStatus(ctx, domainmeeting.StatusEvent{
		MeetingID:    m.ID,
		UserID:       m.UserID,
		Status:       m.Status,
		ErrorMessage: m.ErrorMessage,
	})
}

func (uc *ProcessUC) Execute(ctx context.Context, meetingID uuid.UUID) error {
	m, err := uc.repo.Get(ctx, meetingID)
	if err != nil {
		return fmt.Errorf("process get meeting: %w", err)
	}

	switch m.Status {
	case domainmeeting.StatusCompleted:
		return nil // 冪等：已完成
	case domainmeeting.StatusScheduled:
		return fmt.Errorf("meeting %s not ready: audio upload not confirmed", meetingID)
	case domainmeeting.StatusFailed:
		// retry 由 complete-upload 重新觸發時已轉回 pending；此處視為過期任務
		return nil
	}

	if m.Status == domainmeeting.StatusPending {
		if m, err = uc.repo.UpdateStatus(ctx, m.ID, domainmeeting.StatusPending, domainmeeting.StatusTranscribing); err != nil {
			return fmt.Errorf("process to transcribing: %w", err)
		}
		uc.notify(ctx, m)
		slog.InfoContext(ctx, "meeting.process.transcribing", "meeting_id", m.ID)
	}

	// STT 階段：已有 transcript 則跳過（冪等，不重複扣費）
	if m.Status == domainmeeting.StatusTranscribing {
		if m.Transcript == "" {
			audio, size, err := uc.storage.Download(ctx, m.AudioGCSPath)
			if err != nil {
				return fmt.Errorf("process download audio: %w", err)
			}
			result, err := uc.stt.Transcribe(ctx, audio, size, path.Base(m.AudioGCSPath))
			audio.Close()
			if err != nil {
				return fmt.Errorf("process transcribe: %w", err)
			}
			if m, err = uc.repo.SaveTranscript(ctx, m.ID, result.Text, result.DurationSeconds); err != nil {
				return fmt.Errorf("process save transcript: %w", err)
			}
			slog.InfoContext(ctx, "meeting.process.transcript_saved",
				"meeting_id", m.ID, "duration_seconds", result.DurationSeconds)
		}
		if m, err = uc.repo.UpdateStatus(ctx, m.ID, domainmeeting.StatusTranscribing, domainmeeting.StatusAnalyzing); err != nil {
			return fmt.Errorf("process to analyzing: %w", err)
		}
		uc.notify(ctx, m)
	}

	// analyzing 階段：LLM 文件生成於 Phase 9 接入，目前直接完成（僅逐字稿）
	if m, err = uc.repo.SetCompleted(ctx, m.ID); err != nil {
		return fmt.Errorf("process set completed: %w", err)
	}
	uc.notify(ctx, m)
	slog.InfoContext(ctx, "meeting.process.completed", "meeting_id", m.ID)
	return nil
}

// MarkFailed 由 worker 在最後一次重試失敗後呼叫。
func (uc *ProcessUC) MarkFailed(ctx context.Context, meetingID uuid.UUID, cause error) {
	m, err := uc.repo.SetFailed(ctx, meetingID, cause.Error())
	if err != nil {
		slog.ErrorContext(ctx, "meeting.process.mark_failed_error", "meeting_id", meetingID, "err", err)
		return
	}
	uc.notify(ctx, m)
	slog.WarnContext(ctx, "meeting.process.failed", "meeting_id", meetingID, "cause", cause)
}
