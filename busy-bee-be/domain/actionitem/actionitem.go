// Package actionitem 定義行動項的 entity 與 port interface（零外部依賴）。
package actionitem

import (
	"context"
	"errors"
	"strings"
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
	// DueAt 解析後的到期時點（供提醒/行事曆）；無法解析則為 nil。
	DueAt *time.Time
	// RemindedAt 已推播到期提醒的時間；nil 代表尚未提醒。
	RemindedAt *time.Time
	Done       bool
	CreatedAt  time.Time
}

// PendingItem 跨會議待辦清單用（含所屬會議標題）。
type PendingItem struct {
	ActionItem
	MeetingTitle string
}

// Extracted LLM 抽取結果（尚未落庫）。JSON tag 對應 prompt 要求的輸出格式。
// DueText 保留原文說法（如「下週五」）；DueISO 為模型據會議日期推算的絕對日期（YYYY-MM-DD，無則空）。
type Extracted struct {
	Description string `json:"description"`
	Assignee    string `json:"assignee"`
	DueText     string `json:"due"`
	DueISO      string `json:"dueISO"`
}

// DueDate 將 dueISO（YYYY-MM-DD）解析為該日上午 9 點（UTC+8）作為提醒時點；
// 空值或格式錯誤回 nil，代表此行動項不排提醒。用固定時區避免容器缺 tzdata。
func (e Extracted) DueDate() *time.Time {
	s := strings.TrimSpace(e.DueISO)
	if s == "" {
		return nil
	}
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	loc := time.FixedZone("UTC+8", 8*3600)
	t := time.Date(d.Year(), d.Month(), d.Day(), 9, 0, 0, 0, loc)
	return &t
}

// Extraction LLM 對逐字稿的一次分析結果：一句話摘要 + 行動項（同一次呼叫產生）。
type Extraction struct {
	Summary string      `json:"summary"`
	Items   []Extracted `json:"actionItems"`
}

// Repository 行動項存取 port（pgx 實作在 infrastructure/db）。
type Repository interface {
	Insert(ctx context.Context, meetingID, userID uuid.UUID, item Extracted, sortOrder int) (ActionItem, error)
	// InsertManual 使用者手動新增（source='manual'），重跑分析時不會被刪除。
	// assignee 為指派對象（講者代號或名字，可空）。
	InsertManual(ctx context.Context, meetingID, userID uuid.UUID, description, assignee string) (ActionItem, error)
	// DeleteForMeeting 只刪 LLM 抽取的行動項（重抽用）；手動新增項保留。
	DeleteForMeeting(ctx context.Context, meetingID uuid.UUID) error
	ListByMeeting(ctx context.Context, meetingID uuid.UUID) ([]ActionItem, error)
	ListPendingForUser(ctx context.Context, userID uuid.UUID) ([]PendingItem, error)
	// SetDone 查無列（非本人或不存在）回傳 ErrNotFound。
	SetDone(ctx context.Context, id, userID uuid.UUID, done bool) (ActionItem, error)
	// UpdateDescription 修改待辦內容；查無列（非本人或不存在）回傳 ErrNotFound。
	UpdateDescription(ctx context.Context, id, userID uuid.UUID, description string) (ActionItem, error)
}

// ReminderRepository 到期行動項提醒的存取 port（掃描式，比對 meeting.ReminderRepository）。
type ReminderRepository interface {
	// ListDueReminders 到期（due_at <= now）、未完成、未提醒過的行動項（過期太久不再提醒）。
	ListDueReminders(ctx context.Context) ([]PendingItem, error)
	MarkReminded(ctx context.Context, id uuid.UUID) error
}

// Extractor 逐字稿分析 port（Gemini 實作在 infrastructure/llm）：一次呼叫產出摘要 + 行動項。
// now 為相對時限的參考日期（會議日期），供模型把「下週五」推算成絕對日期。
type Extractor interface {
	Extract(ctx context.Context, transcript string, now time.Time) (Extraction, error)
}
