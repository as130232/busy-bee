package meeting

import (
	"context"
	"fmt"
	"log/slog"
	"path"

	"github.com/google/uuid"

	domainartifact "github.com/as130232/busy-bee/busy-bee-be/domain/artifact"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

// ProcessUC 會議處理管線：pending → transcribing（STT）→ analyzing → completed。
// 各階段冪等（ADR-009）：已有產物的階段直接跳過，retry 不重複呼叫外部 API。
// 失敗時回傳錯誤交由 Asynq retry；標記 failed 是 worker 在最後一次重試後的決定（MarkFailed）。
// ProcessDeps ProcessUC 的依賴（皆為 domain ports）。
type ProcessDeps struct {
	Meetings  domainmeeting.Repository
	Storage   domainmeeting.AudioStorage
	STT       domainmeeting.STTClient
	Artifacts domainartifact.Repository
	LLM       domainartifact.LLMClient
	Notifier  domainmeeting.StatusNotifier
}

type ProcessUC struct {
	repo      domainmeeting.Repository
	storage   domainmeeting.AudioStorage
	stt       domainmeeting.STTClient
	artifacts domainartifact.Repository
	llm       domainartifact.LLMClient
	notifier  domainmeeting.StatusNotifier
}

func NewProcessUC(d ProcessDeps) *ProcessUC {
	return &ProcessUC{
		repo: d.Meetings, storage: d.Storage, stt: d.STT,
		artifacts: d.Artifacts, llm: d.LLM, notifier: d.Notifier,
	}
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

	// analyzing 階段：生成 PRD 與 Tech Spec；缺哪份補哪份（冪等，不重複扣費）
	if m.Status == domainmeeting.StatusAnalyzing {
		if err := uc.generateArtifacts(ctx, m); err != nil {
			return err
		}
	}

	if m, err = uc.repo.SetCompleted(ctx, m.ID); err != nil {
		return fmt.Errorf("process set completed: %w", err)
	}
	uc.notify(ctx, m)
	slog.InfoContext(ctx, "meeting.process.completed", "meeting_id", m.ID)
	return nil
}

func (uc *ProcessUC) generateArtifacts(ctx context.Context, m domainmeeting.Meeting) error {
	existing, err := uc.artifacts.ListByMeeting(ctx, m.ID)
	if err != nil {
		return fmt.Errorf("process list artifacts: %w", err)
	}
	has := make(map[domainartifact.Type]bool, len(existing))
	for _, a := range existing {
		has[a.Type] = true
	}

	generators := []struct {
		t   domainartifact.Type
		gen func(context.Context, string) (string, error)
	}{
		{domainartifact.TypePRD, uc.llm.GeneratePRD},
		{domainartifact.TypeTechSpec, uc.llm.GenerateTechSpec},
	}
	for _, g := range generators {
		if has[g.t] {
			continue
		}
		content, err := g.gen(ctx, m.Transcript)
		if err != nil {
			return fmt.Errorf("process generate %s: %w", g.t, err)
		}
		if _, err := uc.artifacts.Upsert(ctx, m.ID, g.t, content); err != nil {
			return fmt.Errorf("process save %s: %w", g.t, err)
		}
		slog.InfoContext(ctx, "meeting.process.artifact_saved", "meeting_id", m.ID, "type", g.t)
	}
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
