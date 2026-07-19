// Package actionitem 定義行動項的 entity 與 port interface（零外部依賴）。
package actionitem

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound 行動項不存在或非本人所有（owner-only，兩者不區分）。
var ErrNotFound = errors.New("action item not found")

// ActionItem 一筆由逐字稿抽取的行動項。
type ActionItem struct {
	ID          uuid.UUID
	MeetingID   uuid.UUID
	UserID      uuid.UUID
	Description string
	Assignee    string
	DueText     string
	Done        bool
	CreatedAt   time.Time
}

// PendingItem 跨會議待辦清單用（含所屬會議標題）。
type PendingItem struct {
	ActionItem
	MeetingTitle string
}

// Extracted LLM 抽取結果（尚未落庫）。JSON tag 對應 prompt 要求的輸出格式。
type Extracted struct {
	Description string `json:"description"`
	Assignee    string `json:"assignee"`
	DueText     string `json:"due"`
}

// Repository 行動項存取 port（pgx 實作在 infrastructure/db）。
type Repository interface {
	Insert(ctx context.Context, meetingID, userID uuid.UUID, item Extracted, sortOrder int) (ActionItem, error)
	DeleteForMeeting(ctx context.Context, meetingID uuid.UUID) error
	ListByMeeting(ctx context.Context, meetingID uuid.UUID) ([]ActionItem, error)
	ListPendingForUser(ctx context.Context, userID uuid.UUID) ([]PendingItem, error)
	// SetDone 查無列（非本人或不存在）回傳 ErrNotFound。
	SetDone(ctx context.Context, id, userID uuid.UUID, done bool) (ActionItem, error)
}

// Extractor 行動項抽取 port（Gemini 實作在 infrastructure/llm）。
type Extractor interface {
	ExtractActionItems(ctx context.Context, transcript string) ([]Extracted, error)
}
