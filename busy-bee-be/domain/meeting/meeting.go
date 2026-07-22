// Package meeting 定義會議 entity、狀態機與相關 port interface（零外部依賴）。
package meeting

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Status 會議處理狀態機：
// scheduled → pending → transcribing → analyzing → completed | failed；failed 可 retry 回 pending。
type Status string

const (
	StatusScheduled    Status = "scheduled"
	StatusPending      Status = "pending"
	StatusTranscribing Status = "transcribing"
	StatusAnalyzing    Status = "analyzing"
	StatusCompleted    Status = "completed"
	StatusFailed       Status = "failed"
)

var transitions = map[Status][]Status{
	StatusScheduled:    {StatusPending},
	StatusPending:      {StatusTranscribing, StatusFailed},
	StatusTranscribing: {StatusAnalyzing, StatusFailed},
	StatusAnalyzing:    {StatusCompleted, StatusFailed},
	StatusFailed:       {StatusPending}, // 手動 retry
	StatusCompleted:    {},              // 終態
}

func (s Status) IsValid() bool {
	_, ok := transitions[s]
	return ok
}

func (s Status) CanTransitionTo(next Status) bool {
	for _, t := range transitions[s] {
		if t == next {
			return true
		}
	}
	return false
}

// Scenario 紀錄情境：決定 AI 產出的結構化區塊模板。
// meeting（會議）為預設；casual（閒聊）著重重點/結論/待辦；interview（面試）著重問答/評估/後續。
// 日後新增情境 = 新 prompt 模板 + section 定義 + 此處常數與 IsValid。
type Scenario string

const (
	ScenarioMeeting   Scenario = "meeting"
	ScenarioCasual    Scenario = "casual"
	ScenarioInterview Scenario = "interview"
)

func (s Scenario) IsValid() bool {
	return s == ScenarioMeeting || s == ScenarioCasual || s == ScenarioInterview
}

// ParseScenario 將字串轉為 Scenario；無效或空值一律回退預設 meeting（不回錯，容忍舊資料）。
func ParseScenario(s string) Scenario {
	sc := Scenario(s)
	if sc.IsValid() {
		return sc
	}
	return ScenarioMeeting
}

// SummarySection AI 依情境產生的一個結構化區塊（純條列，不做巢狀）。
// Type 為穩定機器識別（如 "decisions"）；Title 為顯示標題；Items 為條列內容。
type SummarySection struct {
	Type  string   `json:"type"`
	Title string   `json:"title"`
	Items []string `json:"items"`
}

// Summarizer 依情境產生結構化摘要區塊的 port（Gemini 實作在 infrastructure/llm）。
// 與行動項抽取（actionitem.Extractor）分離，各自一次 LLM 呼叫。
type Summarizer interface {
	Summarize(ctx context.Context, transcript string, scenario Scenario) ([]SummarySection, error)
}

type Meeting struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Title        string
	AudioGCSPath string
	Status       Status
	// Scenario 紀錄情境（會議/閒聊）；預設 meeting，決定 AI 產出的區塊模板。
	Scenario   Scenario
	Transcript string
	// Summary 一句話摘要（TL;DR）；分析階段由 LLM 產生，未處理則為空。
	Summary string
	// SummarySections 依情境產生的結構化摘要區塊；未處理則為空。
	SummarySections []SummarySection
	// TranscriptSegments 分講者逐字稿；供應商支援 diarization 時填入，否則為空。
	TranscriptSegments []TranscriptSegment
	// SpeakerNames 講者代號 → 使用者自訂顯示名（如 {"A":"Ben"}）；限本場會議內。
	SpeakerNames    map[string]string
	DurationSeconds int
	ErrorMessage    string
	ScheduledAt     *time.Time
	RemindBeforeMin int
	ProcessedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Repository 會議資料存取 port（pgx 實作在 infrastructure/db）。
type Repository interface {
	Create(ctx context.Context, m Meeting) (Meeting, error)
	GetForUser(ctx context.Context, id, userID uuid.UUID) (Meeting, error)
	// Get 不帶 user 過濾，僅供 worker 內部使用。
	Get(ctx context.Context, id uuid.UUID) (Meeting, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, from, to Status) (Meeting, error)
	// SaveTranscript 儲存攤平純文字、分講者片段與時長；不支援 diarization 時 segments 傳 nil。
	SaveTranscript(ctx context.Context, id uuid.UUID, transcript string, segments []TranscriptSegment, durationSeconds int) (Meeting, error)
	// SaveSummary 儲存會議一句話摘要（分析階段產生）。
	SaveSummary(ctx context.Context, id uuid.UUID, summary string) (Meeting, error)
	// SaveSummarySections 儲存依情境產生的結構化摘要區塊（分析階段產生）。
	SaveSummarySections(ctx context.Context, id uuid.UUID, sections []SummarySection) (Meeting, error)
	// SetCompleted analyzing → completed，並記錄 processed_at。
	SetCompleted(ctx context.Context, id uuid.UUID) (Meeting, error)
	// SetFailed 處理中任一狀態 → failed，記錄 error_message。
	SetFailed(ctx context.Context, id uuid.UUID, errorMessage string) (Meeting, error)
	// ListUnfinishedIDs 列出未完成（pending/transcribing/analyzing）會議，供復原掃描。
	ListUnfinishedIDs(ctx context.Context) ([]uuid.UUID, error)
	// ListForUser 列出本人會議（新→舊）；search 非空時以關鍵字過濾 title/transcript。
	ListForUser(ctx context.Context, userID uuid.UUID, search string) ([]Meeting, error)
}

// ScheduleParams 排程會議的輸入欄位。
type ScheduleParams struct {
	Title           string
	Scenario        Scenario
	ScheduledAt     time.Time
	RemindBeforeMin int
}

// ScheduleRepository 排程會議專用窄介面（MeetingRepo 一併實作）。
type ScheduleRepository interface {
	CreateScheduled(ctx context.Context, userID uuid.UUID, p ScheduleParams) (Meeting, error)
	// UpdateSchedule 僅 scheduled 狀態且本人可改；會清除 reminded_at（重排提醒）。
	UpdateSchedule(ctx context.Context, id, userID uuid.UUID, p ScheduleParams) (Meeting, error)
}

// ManageRepository 會議管理專用窄介面（MeetingRepo 一併實作）。
type ManageRepository interface {
	// Rename 重新命名（任何狀態，本人限定）；不存在或非本人回 ErrNotFound。
	Rename(ctx context.Context, id, userID uuid.UUID, title string) (Meeting, error)
	// Delete 刪除會議（任何狀態，本人限定），回傳其音檔路徑供清理 GCS；關聯 artifacts/action_items/chunks 由 FK CASCADE 連帶刪除。不存在或非本人回 ErrNotFound。
	Delete(ctx context.Context, id, userID uuid.UUID) (string, error)
	// UpdateSpeakerNames 更新講者代號→顯示名對應（本人限定）；不存在或非本人回 ErrNotFound。
	UpdateSpeakerNames(ctx context.Context, id, userID uuid.UUID, names map[string]string) (Meeting, error)
}

// ReminderRepository 提醒掃描專用窄介面（MeetingRepo 一併實作）。
type ReminderRepository interface {
	// ListDueReminders 到達提醒時間且未提醒過的排程會議（過期 1 小時以上不再提醒）。
	ListDueReminders(ctx context.Context) ([]Meeting, error)
	MarkReminded(ctx context.Context, id uuid.UUID) error
}
