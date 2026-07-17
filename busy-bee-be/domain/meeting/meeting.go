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
	UpdateStatus(ctx context.Context, id uuid.UUID, from, to Status) (Meeting, error)
}
