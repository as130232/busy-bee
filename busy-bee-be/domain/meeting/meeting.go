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

type Meeting struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	Title           string
	AudioGCSPath    string
	Status          Status
	Transcript      string
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
	// Delete 刪除會議（任何狀態，本人限定）；關聯 artifacts/action_items/chunks 由 FK CASCADE 連帶刪除。不存在或非本人回 ErrNotFound。
	Delete(ctx context.Context, id, userID uuid.UUID) error
	// UpdateSpeakerNames 更新講者代號→顯示名對應（本人限定）；不存在或非本人回 ErrNotFound。
	UpdateSpeakerNames(ctx context.Context, id, userID uuid.UUID, names map[string]string) (Meeting, error)
}

// ReminderRepository 提醒掃描專用窄介面（MeetingRepo 一併實作）。
type ReminderRepository interface {
	// ListDueReminders 到達提醒時間且未提醒過的排程會議（過期 1 小時以上不再提醒）。
	ListDueReminders(ctx context.Context) ([]Meeting, error)
	MarkReminded(ctx context.Context, id uuid.UUID) error
}
