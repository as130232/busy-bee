// Package correction 定義 AI 錄音重新校正（F-CORRECT）的 entity、狀態與 port interface（零外部依賴）。
// 每次校正一筆紀錄：含校正前快照（還原用）與逐段斷點 progress（重試不重複扣費）。
package correction

import (
	"context"
	"errors"
	"io"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	"github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusReverted  Status = "reverted"
)

// MaxAttempts 同一場紀錄最多校正次數（按下即計次，含失敗）。
const MaxAttempts = 2

var (
	ErrNotFound       = errors.New("correction: not found")
	ErrStatusConflict = errors.New("correction: status conflict")
	ErrActiveExists   = errors.New("correction: active correction exists")
	// ErrBadOutput 模型輸出無法解析；ExecuteUC 視為該段無修改（不重試、不扣額外費用）。
	ErrBadOutput = errors.New("correction: bad model output")
)

// Patch 一段逐字稿的修正：I 為 meeting.TranscriptSegments 的全域索引。
type Patch struct {
	I    int    `json:"i"`
	Text string `json:"text"`
}

// Chunk 一個處理段：[First, Last] 為 segment 全域索引（含），音訊範圍 [ClipFromMs, ClipToMs]。
type Chunk struct {
	Index      int
	First      int
	Last       int
	ClipFromMs int
	ClipToMs   int
}

// Progress 斷點：已完成段索引（字串化）→ 該段通過驗證的 patches。
type Progress struct {
	Chunks map[string][]Patch `json:"chunks"`
}

// Snapshot 校正前快照，還原時整份寫回。
type Snapshot struct {
	Segments        []meeting.TranscriptSegment
	Transcript      string
	SummarySections []meeting.SummarySection
	Summary         string
}

type Correction struct {
	ID        uuid.UUID
	MeetingID uuid.UUID
	UserID    uuid.UUID
	Attempt   int
	Status    Status
	Progress  Progress
	Original  Snapshot
	// Guidance 使用者校正前的選填補充（錯字對照、補充說明）。
	Guidance     Guidance
	ErrorMessage string
	CreatedAt    time.Time
	CompletedAt  *time.Time
	RevertedAt   *time.Time
}

// IndexedSegment 送給 LLM 的段落：I 為全域索引；Locked（使用者手動修正過）只讀不改。
type IndexedSegment struct {
	I       int
	Segment meeting.TranscriptSegment
	Locked  bool
}

// CorrectorInput 一段送給 LLM 的內容。
type CorrectorInput struct {
	Audio    io.Reader
	Filename string
	// ClipStartMs 音訊片段在整場錄音中的起點；prompt 時間以此換算為片段內相對時間。
	ClipStartMs int
	Context     []meeting.TranscriptSegment // 上一段結尾（唯讀上下文）
	Segments    []IndexedSegment
	// Background 會前資料段落（F-LIVE）；幫助辨認專有名詞，空字串＝無。
	Background string
	// Terms 使用者指定的錯字對照（Segments 已先套用）；Note 使用者補充說明。皆只用於校正，不進報告。
	Terms []meeting.ReplaceRule
	Note  string
}

// Corrector 聽音訊片段修正逐字稿，只回傳有修改的段落（未驗證，由 application 層過濾）。
type Corrector interface {
	CorrectChunk(ctx context.Context, in CorrectorInput) ([]Patch, error)
}

// ClipCutter 從完整音檔剪出 [fromMs, toMs] 片段（本機暫存檔路徑，caller 呼叫 cleanup）。
type ClipCutter interface {
	CutClip(ctx context.Context, srcPath string, fromMs, toMs int) (clipPath string, cleanup func(), err error)
}

// Result 校正完成要落庫的產物。Patches 於落庫交易內套用到「當下」segments（使用者在 LLM 呼叫期間的編輯優先）；
// DeleteItemIDs 只刪仍未勾選、未改內容的 AI 項。
type Result struct {
	Patches         []Patch
	SummarySections []meeting.SummarySection
	Summary         string
	DeleteItemIDs   []uuid.UUID
	InsertItems     []actionitem.Extracted
	InsertSortFrom  int
}

// Repository correction 存取；對使用者的讀取以 userID 過濾（worker 的 Get 以 correction ID 取用）。
type Repository interface {
	// CreateWithSnapshot 原子：計算 attempt、寫快照與使用者補充、insert queued。
	// 已達 MaxAttempts 回 ErrStatusConflict；已有進行中回 ErrActiveExists。
	CreateWithSnapshot(ctx context.Context, meetingID, userID uuid.UUID, snap Snapshot, g Guidance) (Correction, error)
	Get(ctx context.Context, id uuid.UUID) (Correction, error)
	GetLatestForMeeting(ctx context.Context, meetingID, userID uuid.UUID) (Correction, error)
	CountForMeeting(ctx context.Context, meetingID, userID uuid.UUID) (int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, from, to Status, errorMessage string) (Correction, error)
	SaveProgress(ctx context.Context, id uuid.UUID, p Progress) error
	ListUnfinishedIDs(ctx context.Context) ([]uuid.UUID, error)
	// ApplyResult 原子：鎖定當下 segments 套用 patches、summary_sections、summary、行動項替換、status running→completed。
	ApplyResult(ctx context.Context, c Correction, r Result) error
	// Revert 原子：快照寫回 meeting（保留校正後的手動修改，見 RevertSegments）、status completed→reverted。
	Revert(ctx context.Context, c Correction) error
}

// State 對外狀態（API 用）。
type State struct {
	Status       Status
	AttemptsUsed int
	CanRevert    bool
	ErrorMessage string
}

// StateFromLatest 由最新一筆與總筆數推導對外狀態。
func StateFromLatest(latest *Correction, count int) State {
	if latest == nil {
		return State{}
	}
	return State{
		Status:       latest.Status,
		AttemptsUsed: count,
		CanRevert:    latest.Status == StatusCompleted,
		ErrorMessage: latest.ErrorMessage,
	}
}

// ApplyPatches 回傳套用後的新 slice（不改輸入）：
// 此刻已 ManuallyEdited 的段跳過（校正期間使用者編輯優先）；OriginalText 只在首次被改時寫入；清空 MarkedText。
func ApplyPatches(current []meeting.TranscriptSegment, patches []Patch) []meeting.TranscriptSegment {
	out := slices.Clone(current)
	for _, p := range patches {
		if p.I < 0 || p.I >= len(out) || out[p.I].ManuallyEdited {
			continue
		}
		s := &out[p.I]
		if s.OriginalText == "" {
			s.OriginalText = s.Text
		}
		s.Text = p.Text
		s.MarkedText = ""
	}
	return out
}

// RevertSegments 還原用：以快照為準，但保留「校正後」使用者手動修改的段落（ManuallyEdited 且文字與快照不同），
// 並清掉其 OriginalText（已非 AI 校正結果）。段數不一致（理論上不會發生）時整份用快照。
func RevertSegments(snapshot, current []meeting.TranscriptSegment) []meeting.TranscriptSegment {
	out := slices.Clone(snapshot)
	if len(current) != len(snapshot) {
		return out
	}
	for i, cur := range current {
		if cur.ManuallyEdited && cur.Text != snapshot[i].Text {
			cur.OriginalText = ""
			out[i] = cur
		}
	}
	return out
}
