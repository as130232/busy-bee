package correction

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	"github.com/as130232/busy-bee/busy-bee-be/domain/correction"
	"github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainmc "github.com/as130232/busy-bee/busy-bee-be/domain/meetingcontext"
)

// failedMessage 使用者可見的失敗訊息；原始 cause 只進 log。
const failedMessage = "AI 校正失敗，請稍後再試"

// ExecuteDeps ExecuteUC 的依賴（皆為 port）。
type ExecuteDeps struct {
	Repo       correction.Repository
	Meetings   MeetingReader
	Storage    AudioDownloader
	Clipper    correction.ClipCutter
	Corrector  correction.Corrector
	Summarizer meeting.Summarizer
	Extractor  actionitem.Extractor
	Items      ActionItemLister
	Indexer    Indexer
	Notifier   meeting.StatusNotifier
	// Contexts 會前資料（F-LIVE）；選填，nil 時無背景資料。
	Contexts ContextLister
	// Compress 將下載的音檔壓成本機暫存檔（stt.CompressToTempFile），供 Clipper 剪片段。
	Compress func(ctx context.Context, r io.Reader) (path string, cleanup func(), err error)
	// OpenClip 開啟剪好的片段檔；nil 時用 os.Open。
	OpenClip func(path string) (io.ReadCloser, error)
}

// ExecuteUC 校正 worker：逐段聽音訊修正逐字稿 → 套用 → 重產摘要 → 合併行動項 → 重建索引。
// 冪等（ADR-009）：終態直接結束；每段完成即寫 progress，重試從斷點接續、不重複扣費。
type ExecuteUC struct {
	d ExecuteDeps
}

func NewExecuteUC(d ExecuteDeps) *ExecuteUC {
	if d.OpenClip == nil {
		d.OpenClip = func(p string) (io.ReadCloser, error) { return os.Open(p) }
	}
	return &ExecuteUC{d: d}
}

func (uc *ExecuteUC) Execute(ctx context.Context, id uuid.UUID) error {
	c, err := uc.d.Repo.Get(ctx, id)
	if errors.Is(err, correction.ErrNotFound) {
		return nil // 紀錄已刪（cascade）
	}
	if err != nil {
		return fmt.Errorf("correction get: %w", err)
	}
	switch c.Status {
	case correction.StatusCompleted, correction.StatusFailed, correction.StatusReverted:
		return nil
	case correction.StatusQueued:
		if c, err = uc.d.Repo.UpdateStatus(ctx, id, correction.StatusQueued, correction.StatusRunning, ""); err != nil {
			return fmt.Errorf("correction start running: %w", err)
		}
		uc.notify(ctx, c)
	}

	m, err := uc.d.Meetings.GetForUser(ctx, c.MeetingID, c.UserID)
	if errors.Is(err, meeting.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("correction get meeting: %w", err)
	}

	bg, err := uc.background(ctx, m)
	if err != nil {
		return err
	}

	// 切段以快照 segments 為準（索引穩定）；套用時才讀最新 segments。
	// 使用者的錯字對照先套在快照上再送模型；模型輸出再套一次，確保對照一定生效。
	base := c.Guidance.Apply(c.Original.Segments)
	llmPatches, err := uc.correctAll(ctx, &c, m.AudioGCSPath, base, bg)
	if err != nil {
		return err
	}
	all := GuidancePatches(c.Original.Segments, base, llmPatches, c.Guidance)

	m, err = uc.d.Meetings.GetForUser(ctx, c.MeetingID, c.UserID) // 重讀：校正期間的人工編輯優先
	if err != nil {
		return fmt.Errorf("correction reload meeting: %w", err)
	}
	segs := correction.ApplyPatches(m.TranscriptSegments, all)

	// 摘要回傳的 tags 忽略：tags 與手動標籤共用欄位，重寫會蓋掉使用者標籤（spec D2a）
	sum, err := uc.d.Summarizer.Summarize(ctx, meeting.TimestampedTranscript(segs, m.Transcript), m.Scenario, bg)
	if err != nil {
		return fmt.Errorf("correction summarize: %w", err)
	}
	plan, summary, err := uc.planActionItems(ctx, m, segs, bg)
	if err != nil {
		return err
	}

	if err := uc.d.Repo.ApplyResult(ctx, c, correction.Result{
		Patches:         all,
		SummarySections: sum.Sections, Summary: summary,
		DeleteItemIDs: plan.DeleteIDs, InsertItems: plan.Insert, InsertSortFrom: plan.InsertSortFrom,
	}); err != nil {
		return fmt.Errorf("correction apply: %w", err)
	}
	if err := uc.d.Indexer.Execute(ctx, m.ID); err != nil {
		slog.WarnContext(ctx, "correction.reindex_failed", "meeting_id", m.ID, "err", err) // 回填掃描會補
	}
	c.Status = correction.StatusCompleted
	uc.notify(ctx, c)
	slog.InfoContext(ctx, "correction.completed", "meeting_id", m.ID, "correction_id", c.ID, "patches", len(all))
	return nil
}

// correctAll 逐段校正，回傳全部通過驗證的 patches；progress 已有的段直接沿用。
func (uc *ExecuteUC) correctAll(ctx context.Context, c *correction.Correction, audioPath string, base []meeting.TranscriptSegment, bg string) ([]correction.Patch, error) {
	chunks := PlanChunks(base)
	var all []correction.Patch
	var fullAudio string
	for _, ch := range chunks {
		key := strconv.Itoa(ch.Index)
		if done, ok := c.Progress.Chunks[key]; ok {
			all = append(all, done...)
			continue
		}
		if fullAudio == "" { // 只有真的有段要處理才下載（斷點全完成時免下載）
			p, cleanup, err := uc.prepareAudio(ctx, audioPath)
			if err != nil {
				return nil, err
			}
			defer cleanup()
			fullAudio = p
		}
		patches, err := uc.correctChunk(ctx, fullAudio, base, all, ch, bg, c.Guidance)
		if err != nil {
			return nil, err
		}
		c.Progress.Chunks[key] = patches
		if err := uc.d.Repo.SaveProgress(ctx, c.ID, c.Progress); err != nil {
			return nil, fmt.Errorf("correction save progress: %w", err)
		}
		all = append(all, patches...)
	}
	return all, nil
}

func (uc *ExecuteUC) prepareAudio(ctx context.Context, objectPath string) (string, func(), error) {
	rc, _, err := uc.d.Storage.Download(ctx, objectPath)
	if err != nil {
		return "", nil, fmt.Errorf("correction download audio: %w", err)
	}
	defer rc.Close()
	p, cleanup, err := uc.d.Compress(ctx, rc)
	if err != nil {
		return "", nil, fmt.Errorf("correction compress audio: %w", err)
	}
	return p, cleanup, nil
}

// correctChunk 剪片段 → 組上下文（上一段結尾，已套用先前 patch）與待校正段 → 呼叫模型 → 驗證。
// 模型輸出無法解析時視為該段無修改（不重試）；其他錯誤回傳讓佇列重試。
func (uc *ExecuteUC) correctChunk(ctx context.Context, audioPath string, base []meeting.TranscriptSegment, prior []correction.Patch, ch correction.Chunk, bg string, g correction.Guidance) ([]correction.Patch, error) {
	clipPath, cleanup, err := uc.d.Clipper.CutClip(ctx, audioPath, ch.ClipFromMs, ch.ClipToMs)
	if err != nil {
		return nil, fmt.Errorf("correction cut clip %d: %w", ch.Index, err)
	}
	defer cleanup()
	clip, err := uc.d.OpenClip(clipPath)
	if err != nil {
		return nil, fmt.Errorf("correction open clip %d: %w", ch.Index, err)
	}
	defer clip.Close()

	patched := correction.ApplyPatches(base, prior)
	in := correction.CorrectorInput{
		Audio:       clip,
		Filename:    "clip.mp3",
		ClipStartMs: ch.ClipFromMs,
		Context:     patched[max(0, ch.First-contextSegments):ch.First],
		Background:  bg,
		Terms:       g.Replacements,
		Note:        g.Note,
	}
	for i := ch.First; i <= ch.Last; i++ {
		in.Segments = append(in.Segments, correction.IndexedSegment{I: i, Segment: base[i], Locked: base[i].ManuallyEdited})
	}
	out, err := uc.d.Corrector.CorrectChunk(ctx, in)
	if errors.Is(err, correction.ErrBadOutput) {
		slog.WarnContext(ctx, "correction.chunk.bad_output", "chunk", ch.Index, "err", err)
		return []correction.Patch{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("correction chunk %d: %w", ch.Index, err)
	}
	return ValidatePatches(base, ch, out), nil
}

// planActionItems 以校正後逐字稿重抽行動項（告知模型保留項已存在），回傳合併計畫與新一句話摘要。
func (uc *ExecuteUC) planActionItems(ctx context.Context, m meeting.Meeting, segs []meeting.TranscriptSegment, bg string) (MergePlan, string, error) {
	items, err := uc.d.Items.ListByMeeting(ctx, m.ID)
	if err != nil {
		return MergePlan{}, "", fmt.Errorf("correction list items: %w", err)
	}
	kept, repl := SplitActionItems(items)
	existing := make([]string, len(kept))
	for i, k := range kept {
		existing[i] = k.Description
	}
	ext, err := uc.d.Extractor.ExtractExcluding(ctx, meeting.AnalysisTranscript(segs, m.Transcript), m.CreatedAt, existing, bg)
	if err != nil {
		return MergePlan{}, "", fmt.Errorf("correction extract: %w", err)
	}
	return PlanMerge(kept, repl, ext.Items), strings.TrimSpace(ext.Summary), nil
}

// background 會前資料 prompt 段落；未注入 Contexts 或無資料時為空字串。
func (uc *ExecuteUC) background(ctx context.Context, m meeting.Meeting) (string, error) {
	if uc.d.Contexts == nil {
		return "", nil
	}
	cs, err := uc.d.Contexts.List(ctx, m.ID, m.UserID)
	if err != nil {
		return "", fmt.Errorf("correction list contexts: %w", err)
	}
	return domainmc.PromptBlock(cs), nil
}

// MarkFailed 佇列重試用盡後呼叫：標 failed（使用者可見訊息不含 cause）並發布事件。
func (uc *ExecuteUC) MarkFailed(ctx context.Context, id uuid.UUID, cause error) {
	slog.ErrorContext(ctx, "correction.failed", "correction_id", id, "err", cause)
	var c correction.Correction
	var err error
	for _, from := range []correction.Status{correction.StatusRunning, correction.StatusQueued} {
		if c, err = uc.d.Repo.UpdateStatus(ctx, id, from, correction.StatusFailed, failedMessage); err == nil {
			uc.notify(ctx, c)
			return
		}
	}
	slog.WarnContext(ctx, "correction.mark_failed_skipped", "correction_id", id, "err", err)
}

func (uc *ExecuteUC) notify(ctx context.Context, c correction.Correction) {
	uc.d.Notifier.NotifyStatus(ctx, meeting.StatusEvent{
		MeetingID: c.MeetingID, UserID: c.UserID, Status: meeting.StatusCompleted,
		CorrectionStatus: string(c.Status),
	})
}
