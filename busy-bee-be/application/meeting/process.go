package meeting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strings"

	"github.com/google/uuid"

	domainactionitem "github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	domainartifact "github.com/as130232/busy-bee/busy-bee-be/domain/artifact"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

// artifactTypeActionItems：抽取階段以此類型的 artifact（原始 JSON）作為冪等標記，
// 存在即代表本場已抽取過，retry 不重複呼叫 LLM。
const artifactTypeActionItems = domainartifact.Type("action_items")

// countSpeakers 計算片段中不重複的講者數（觀測用）。
func countSpeakers(segs []domainmeeting.TranscriptSegment) int {
	set := make(map[string]struct{}, len(segs))
	for _, s := range segs {
		set[s.Speaker] = struct{}{}
	}
	return len(set)
}

// MeetingIndexer 語意索引窄介面（*application/search.IndexUC 滿足）。
// completed 後 best-effort 觸發；索引失敗不回退 completed，保底靠 worker 回填掃描。
type MeetingIndexer interface {
	Execute(ctx context.Context, meetingID uuid.UUID) error
}

// ProcessUC 會議處理管線：pending → transcribing（STT）→ analyzing → completed。
// 各階段冪等（ADR-009）：已有產物的階段直接跳過，retry 不重複呼叫外部 API。
// 失敗時回傳錯誤交由 Asynq retry；標記 failed 是 worker 在最後一次重試後的決定（MarkFailed）。
// ProcessDeps ProcessUC 的依賴（皆為 domain ports）。
type ProcessDeps struct {
	Meetings    domainmeeting.Repository
	Storage     domainmeeting.AudioStorage
	STT         domainmeeting.STTClient
	Artifacts   domainartifact.Repository
	LLM         domainartifact.LLMClient
	Summarizer  domainmeeting.Summarizer
	Notifier    domainmeeting.StatusNotifier
	ActionItems domainactionitem.Repository
	Extractor   domainactionitem.Extractor
	Indexer     MeetingIndexer // 選填；nil 時跳過語意索引
}

type ProcessUC struct {
	repo        domainmeeting.Repository
	storage     domainmeeting.AudioStorage
	stt         domainmeeting.STTClient
	artifacts   domainartifact.Repository
	llm         domainartifact.LLMClient
	summarizer  domainmeeting.Summarizer
	notifier    domainmeeting.StatusNotifier
	actionItems domainactionitem.Repository
	extractor   domainactionitem.Extractor
	indexer     MeetingIndexer
}

func NewProcessUC(d ProcessDeps) *ProcessUC {
	return &ProcessUC{
		repo: d.Meetings, storage: d.Storage, stt: d.STT,
		artifacts: d.Artifacts, llm: d.LLM, summarizer: d.Summarizer, notifier: d.Notifier,
		actionItems: d.ActionItems, extractor: d.Extractor,
		indexer: d.Indexer,
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
		if errors.Is(err, domainmeeting.ErrNotFound) {
			return nil // 會議已被刪除，任務作廢（不重試、不記錯誤）
		}
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
			// 有分講者片段時，攤平成帶講者前綴的文字（供 LLM 分析與搜尋沿用）；否則用原始純文字。
			transcript := result.Text
			if len(result.Segments) > 0 {
				transcript = domainmeeting.FlattenSegments(result.Segments)
			}
			// 空結果保護：STT 轉不出任何文字時視為失敗（可重試），避免靜默完成並用空稿生成垃圾產物。
			if strings.TrimSpace(transcript) == "" {
				return fmt.Errorf("process transcribe: empty transcript (STT 未轉出內容，檢查音檔或供應商語言設定)")
			}
			if m, err = uc.repo.SaveTranscript(ctx, m.ID, transcript, result.Segments, result.DurationSeconds); err != nil {
				return fmt.Errorf("process save transcript: %w", err)
			}
			slog.InfoContext(ctx, "meeting.process.transcript_saved",
				"meeting_id", m.ID, "duration_seconds", result.DurationSeconds,
				"segments", len(result.Segments), "speakers", countSpeakers(result.Segments))
		}
		if m, err = uc.repo.UpdateStatus(ctx, m.ID, domainmeeting.StatusTranscribing, domainmeeting.StatusAnalyzing); err != nil {
			return fmt.Errorf("process to analyzing: %w", err)
		}
		uc.notify(ctx, m)
	}

	// analyzing 階段：依情境產生結構化摘要區塊 + 抽取行動項（各自冪等，不重複扣費）。
	// PRD / Tech Spec 已改為選用、不再自動產生（generateArtifacts 保留供日後 on-demand 觸發）。
	if m.Status == domainmeeting.StatusAnalyzing {
		if err := uc.generateSummarySections(ctx, m); err != nil {
			return err
		}
		if err := uc.extractActionItems(ctx, m); err != nil {
			return err
		}
	}

	if m, err = uc.repo.SetCompleted(ctx, m.ID); err != nil {
		return fmt.Errorf("process set completed: %w", err)
	}
	uc.notify(ctx, m)
	slog.InfoContext(ctx, "meeting.process.completed", "meeting_id", m.ID)

	// best-effort 語意索引：失敗不回退 completed，保底靠 worker 回填掃描
	if uc.indexer != nil {
		if err := uc.indexer.Execute(ctx, m.ID); err != nil {
			slog.WarnContext(ctx, "meeting.process.index_failed", "meeting_id", m.ID, "err", err)
		}
	}
	return nil
}

// generateSummarySections 依情境（會議/閒聊）產生結構化摘要區塊並落庫。
// 冪等（ADR-009）：meeting 已有 summary_sections 則跳過，不重複呼叫 LLM。
func (uc *ProcessUC) generateSummarySections(ctx context.Context, m domainmeeting.Meeting) error {
	if len(m.SummarySections) > 0 {
		return nil // 已產生
	}
	sections, err := uc.summarizer.Summarize(ctx, m.Transcript, m.Scenario)
	if err != nil {
		return fmt.Errorf("process summarize: %w", err)
	}
	if len(sections) == 0 {
		return nil // 無區塊可存；不覆寫成空（空稿已於 STT 階段擋下）
	}
	if _, err := uc.repo.SaveSummarySections(ctx, m.ID, sections); err != nil {
		return fmt.Errorf("process save summary sections: %w", err)
	}
	slog.InfoContext(ctx, "meeting.process.summary_sections_saved",
		"meeting_id", m.ID, "scenario", m.Scenario, "sections", len(sections))
	return nil
}

// generateArtifacts 產生 PRD 與 Tech Spec（缺哪份補哪份，冪等）。
// 已不在預設管線自動觸發；保留供日後 on-demand（進階）產生使用。
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

// extractActionItems 從逐字稿抽取行動項並落庫。
// 冪等：artifacts 表已有 action_items 標記則跳過（不重複呼叫 LLM）。
// 順序為 delete → insert → 寫標記；中途失敗交由 retry 重抽（極端情況多付一次 LLM，但不產生重複列）。
func (uc *ProcessUC) extractActionItems(ctx context.Context, m domainmeeting.Meeting) error {
	existing, err := uc.artifacts.ListByMeeting(ctx, m.ID)
	if err != nil {
		return fmt.Errorf("process list artifacts for action items: %w", err)
	}
	for _, a := range existing {
		if a.Type == artifactTypeActionItems {
			return nil // 已抽取過
		}
	}

	// 同一次呼叫產出摘要 + 行動項（A 方案：不增加額外 LLM 呼叫）。
	result, err := uc.extractor.Extract(ctx, m.Transcript)
	if err != nil {
		return fmt.Errorf("process extract: %w", err)
	}
	items := result.Items

	if summary := strings.TrimSpace(result.Summary); summary != "" {
		if _, err := uc.repo.SaveSummary(ctx, m.ID, summary); err != nil {
			return fmt.Errorf("process save summary: %w", err)
		}
	}

	if err := uc.actionItems.DeleteForMeeting(ctx, m.ID); err != nil {
		return fmt.Errorf("process clear action items: %w", err)
	}
	for i, it := range items {
		if _, err := uc.actionItems.Insert(ctx, m.ID, m.UserID, it, i); err != nil {
			return fmt.Errorf("process insert action item: %w", err)
		}
	}

	raw, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("process marshal action items: %w", err)
	}
	if _, err := uc.artifacts.Upsert(ctx, m.ID, artifactTypeActionItems, string(raw)); err != nil {
		return fmt.Errorf("process mark action items extracted: %w", err)
	}
	slog.InfoContext(ctx, "meeting.process.extracted", "meeting_id", m.ID,
		"action_items", len(items), "has_summary", result.Summary != "")
	return nil
}

// failedUserMessage 寫入 error_message 並經 API / WS 回給前端；
// 外部錯誤原文只進 log，禁止暴露給用戶端（資料安全規範）。
const failedUserMessage = "會議處理失敗，請重試"

// MarkFailed 由 worker 在最後一次重試失敗後呼叫。
func (uc *ProcessUC) MarkFailed(ctx context.Context, meetingID uuid.UUID, cause error) {
	m, err := uc.repo.SetFailed(ctx, meetingID, failedUserMessage)
	if err != nil {
		slog.ErrorContext(ctx, "meeting.process.mark_failed_error", "meeting_id", meetingID, "err", err)
		return
	}
	uc.notify(ctx, m)
	slog.WarnContext(ctx, "meeting.process.failed", "meeting_id", meetingID, "cause", cause)
}
