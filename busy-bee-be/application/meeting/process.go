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

// timestampedTranscript 將分講者片段組成帶時間錨點的逐字稿，供摘要 LLM 標註每個重點的音檔位置。
// 每行格式：[t=<起始毫秒>] <講者>: <內容>。無片段（未 diarize）時退回純逐字稿 fallback。
func timestampedTranscript(segs []domainmeeting.TranscriptSegment, fallback string) string {
	if len(segs) == 0 {
		return fallback
	}
	var b strings.Builder
	for _, s := range segs {
		if s.Speaker != "" {
			fmt.Fprintf(&b, "[t=%d] %s: %s\n", s.StartMs, s.Speaker, s.Text)
		} else {
			fmt.Fprintf(&b, "[t=%d] %s\n", s.StartMs, s.Text)
		}
	}
	return b.String()
}

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

// ExtractionSaver 於單一 transaction 內原子落庫抽取階段產物（摘要 + 行動項 + 冪等標記）。
// 由 infrastructure/db.ProcessRepo 實作；確保中途失敗整批 rollback、不留部分/空狀態。
type ExtractionSaver interface {
	SaveExtraction(ctx context.Context, meetingID, userID uuid.UUID, summary string, items []domainactionitem.Extracted, markerType domainartifact.Type, markerJSON string) error
}

// ProcessUC 會議處理管線：pending → transcribing（STT）→ analyzing → completed。
// 各階段冪等（ADR-009）：已有產物的階段直接跳過，retry 不重複呼叫外部 API。
// 失敗時回傳錯誤交由 Asynq retry；標記 failed 是 worker 在最後一次重試後的決定（MarkFailed）。
// ProcessDeps ProcessUC 的依賴（皆為 domain ports）。
type ProcessDeps struct {
	Meetings   domainmeeting.Repository
	Storage    domainmeeting.AudioStorage
	STT        domainmeeting.STTClient
	Artifacts  domainartifact.Repository
	Summarizer domainmeeting.Summarizer
	Notifier   domainmeeting.StatusNotifier
	Extractor  domainactionitem.Extractor
	Saver      ExtractionSaver
	Indexer    MeetingIndexer             // 選填；nil 時跳過語意索引
	Fetcher    domainmeeting.AudioFetcher // 選填；匯入來源（SourceURL）抓取音訊，nil 時跳過抓取階段
	Renamer    TitleRenamer               // 選填；匯入抓取後以來源標題更新會議標題
	TagWriter  TagWriter                  // 選填；分析階段寫入 AI 自動標籤
}

// TitleRenamer 匯入抓取後以來源標題（如 YouTube 影片名）更新會議標題（ISP，*db.MeetingRepo 滿足）。
type TitleRenamer interface {
	Rename(ctx context.Context, id, userID uuid.UUID, title string) (domainmeeting.Meeting, error)
}

// TagWriter 分析階段寫入 AI 自動標籤（ISP，*db.MeetingRepo 滿足）。
type TagWriter interface {
	UpdateTags(ctx context.Context, id, userID uuid.UUID, tags []string) (domainmeeting.Meeting, error)
}

type ProcessUC struct {
	repo       domainmeeting.Repository
	storage    domainmeeting.AudioStorage
	stt        domainmeeting.STTClient
	artifacts  domainartifact.Repository
	summarizer domainmeeting.Summarizer
	notifier   domainmeeting.StatusNotifier
	extractor  domainactionitem.Extractor
	saver      ExtractionSaver
	indexer    MeetingIndexer
	fetcher    domainmeeting.AudioFetcher
	renamer    TitleRenamer
	tagWriter  TagWriter
}

func NewProcessUC(d ProcessDeps) *ProcessUC {
	return &ProcessUC{
		repo: d.Meetings, storage: d.Storage, stt: d.STT,
		artifacts: d.Artifacts, summarizer: d.Summarizer, notifier: d.Notifier,
		extractor: d.Extractor, saver: d.Saver,
		indexer: d.Indexer, fetcher: d.Fetcher, renamer: d.Renamer, tagWriter: d.TagWriter,
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

	// 匯入來源：pending 且尚無音檔則先抓取上傳 GCS（冪等，已有音檔跳過），再接原管線。
	if m.Status == domainmeeting.StatusPending && m.SourceURL != "" && uc.fetcher != nil {
		if m, err = uc.fetchStage(ctx, m); err != nil {
			return err
		}
	}

	if m.Status == domainmeeting.StatusPending {
		if m, err = uc.repo.UpdateStatus(ctx, m.ID, domainmeeting.StatusPending, domainmeeting.StatusTranscribing); err != nil {
			return fmt.Errorf("process to transcribing: %w", err)
		}
		uc.notify(ctx, m)
		slog.InfoContext(ctx, "meeting.process.transcribing", "meeting_id", m.ID)
	}

	if m.Status == domainmeeting.StatusTranscribing {
		if m, err = uc.transcribeStage(ctx, m); err != nil {
			return err
		}
	}

	if m.Status == domainmeeting.StatusAnalyzing {
		if err := uc.analyzeStage(ctx, m); err != nil {
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

// fetchStage 匯入來源：GCS 尚無音檔則抓取上傳（冪等，已抓過跳過），並以來源標題更新會議名（best-effort）。
func (uc *ProcessUC) fetchStage(ctx context.Context, m domainmeeting.Meeting) (domainmeeting.Meeting, error) {
	exists, err := uc.storage.Exists(ctx, m.AudioGCSPath)
	if err != nil {
		return m, fmt.Errorf("process fetch exists: %w", err)
	}
	if exists {
		return m, nil // 冪等：已抓取上傳過
	}

	fetched, err := uc.fetcher.Fetch(ctx, m.SourceURL)
	if err != nil {
		return m, fmt.Errorf("process fetch source: %w", err)
	}
	uploadErr := uc.storage.Upload(ctx, m.AudioGCSPath, fetched.Reader, fetched.ContentType)
	_ = fetched.Reader.Close()
	if uploadErr != nil {
		return m, fmt.Errorf("process fetch upload: %w", uploadErr)
	}
	slog.InfoContext(ctx, "meeting.process.fetched",
		"meeting_id", m.ID, "size", fetched.Size, "duration_sec", fetched.DurationSec)

	// 以來源標題更新（使用者未自訂、仍是匯入預設名時才覆蓋）。
	if uc.renamer != nil && fetched.Title != "" && (m.Title == "" || m.Title == importPlaceholderTitle) {
		if renamed, rerr := uc.renamer.Rename(ctx, m.ID, m.UserID, fetched.Title); rerr == nil {
			m = renamed
		} else {
			slog.WarnContext(ctx, "meeting.process.fetch_rename_failed", "meeting_id", m.ID, "err", rerr)
		}
	}
	return m, nil
}

// transcribeStage STT 階段：已有 transcript 則跳過轉錄（冪等，不重複扣費），最後轉狀態到 analyzing。
func (uc *ProcessUC) transcribeStage(ctx context.Context, m domainmeeting.Meeting) (domainmeeting.Meeting, error) {
	if m.Transcript == "" {
		result, err := uc.transcribe(ctx, m)
		if err != nil {
			return m, err
		}
		// 有分講者片段時，攤平成帶講者前綴的文字（供 LLM 分析與搜尋沿用）；否則用原始純文字。
		transcript := result.Text
		if len(result.Segments) > 0 {
			transcript = domainmeeting.FlattenSegments(result.Segments)
		}
		// 空結果保護：STT 轉不出任何文字時視為失敗（可重試），避免靜默完成並用空稿生成垃圾產物。
		if strings.TrimSpace(transcript) == "" {
			return m, fmt.Errorf("process transcribe: empty transcript (STT 未轉出內容，檢查音檔或供應商語言設定)")
		}
		if m, err = uc.repo.SaveTranscript(ctx, m.ID, m.UserID, transcript, result.Segments, result.DurationSeconds); err != nil {
			return m, fmt.Errorf("process save transcript: %w", err)
		}
		slog.InfoContext(ctx, "meeting.process.transcript_saved",
			"meeting_id", m.ID, "duration_seconds", result.DurationSeconds,
			"segments", len(result.Segments), "speakers", countSpeakers(result.Segments))
	}

	m, err := uc.repo.UpdateStatus(ctx, m.ID, domainmeeting.StatusTranscribing, domainmeeting.StatusAnalyzing)
	if err != nil {
		return m, fmt.Errorf("process to analyzing: %w", err)
	}
	uc.notify(ctx, m)
	return m, nil
}

// transcribe 下載音檔並呼叫 STT。audio reader 在本函式結束時關閉（defer 作用域限縮於此，
// Transcribe panic 時仍會關閉，避免 GCS reader 洩漏）。
func (uc *ProcessUC) transcribe(ctx context.Context, m domainmeeting.Meeting) (domainmeeting.TranscribeResult, error) {
	audio, size, err := uc.storage.Download(ctx, m.AudioGCSPath)
	if err != nil {
		return domainmeeting.TranscribeResult{}, fmt.Errorf("process download audio: %w", err)
	}
	defer audio.Close()

	result, err := uc.stt.Transcribe(ctx, audio, size, path.Base(m.AudioGCSPath))
	if err != nil {
		return domainmeeting.TranscribeResult{}, fmt.Errorf("process transcribe: %w", err)
	}
	return result, nil
}

// analyzeStage analyzing 階段：依情境產生結構化摘要區塊 + 抽取行動項（各自冪等，不重複扣費）。
func (uc *ProcessUC) analyzeStage(ctx context.Context, m domainmeeting.Meeting) error {
	if err := uc.generateSummarySections(ctx, m); err != nil {
		return err
	}
	return uc.extractActionItems(ctx, m)
}

// generateSummarySections 依情境（會議/閒聊）產生結構化摘要區塊並落庫。
// 冪等（ADR-009）：meeting 已有 summary_sections 則跳過，不重複呼叫 LLM。
func (uc *ProcessUC) generateSummarySections(ctx context.Context, m domainmeeting.Meeting) error {
	if len(m.SummarySections) > 0 {
		return nil // 已產生
	}
	// 餵帶時間錨點的逐字稿（[t=毫秒]），讓 LLM 為每個重點標 startMs 供跳轉音檔。
	res, err := uc.summarizer.Summarize(ctx, timestampedTranscript(m.TranscriptSegments, m.Transcript), m.Scenario)
	if err != nil {
		return fmt.Errorf("process summarize: %w", err)
	}
	if len(res.Sections) == 0 {
		return nil // 無區塊可存；不覆寫成空（空稿已於 STT 階段擋下）
	}
	if _, err := uc.repo.SaveSummarySections(ctx, m.ID, m.UserID, res.Sections); err != nil {
		return fmt.Errorf("process save summary sections: %w", err)
	}
	slog.InfoContext(ctx, "meeting.process.summary_sections_saved",
		"meeting_id", m.ID, "scenario", m.Scenario, "sections", len(res.Sections))

	// AI 自動標籤（best-effort，與手動標籤共用 tags 欄位）。此時仍在處理中、尚無手動標籤，故直接覆寫。
	if uc.tagWriter != nil {
		if tags := cleanAutoTags(res.Tags); len(tags) > 0 {
			if _, terr := uc.tagWriter.UpdateTags(ctx, m.ID, m.UserID, tags); terr != nil {
				slog.WarnContext(ctx, "meeting.process.autotag_failed", "meeting_id", m.ID, "err", terr)
			}
		}
	}
	return nil
}

// cleanAutoTags 清洗 AI 標籤：去空白、截長、去重、上限 5 個。
func cleanAutoTags(tags []string) []string {
	const maxAutoTags = 5
	out := make([]string, 0, len(tags))
	seen := make(map[string]bool, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if r := []rune(t); len(r) > 30 {
			t = string(r[:30])
		}
		if seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
		if len(out) >= maxAutoTags {
			break
		}
	}
	return out
}

// extractActionItems 從逐字稿抽取行動項並落庫。
// 冪等：artifacts 表已有 action_items 標記則跳過（不重複呼叫 LLM）。
// 摘要 + 清空重寫行動項 + 寫標記交由 Saver 於單一 transaction 完成（原子性，見 ExtractionSaver）；
// 中途失敗整批 rollback，retry 重抽（極端情況多付一次 LLM，但不產生重複列或瞬間空清單）。
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
	// 以會議建立時間為參考日期，供模型把相對時限推算成絕對日期。
	result, err := uc.extractor.Extract(ctx, m.Transcript, m.CreatedAt)
	if err != nil {
		return fmt.Errorf("process extract: %w", err)
	}
	items := result.Items

	raw, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("process marshal action items: %w", err)
	}
	if err := uc.saver.SaveExtraction(ctx, m.ID, m.UserID, strings.TrimSpace(result.Summary), items, artifactTypeActionItems, string(raw)); err != nil {
		return fmt.Errorf("process save extraction: %w", err)
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
