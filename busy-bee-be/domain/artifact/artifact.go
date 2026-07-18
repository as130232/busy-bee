// Package artifact 定義 AI 產出文件的 entity 與 port interface（零外部依賴）。
package artifact

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Type 文件類型。
type Type string

const (
	TypePRD      Type = "prd"
	TypeTechSpec Type = "tech_spec"
)

// Artifact 一份由逐字稿生成的結構化 Markdown 文件。
type Artifact struct {
	ID        uuid.UUID
	MeetingID uuid.UUID
	Type      Type
	Content   string
	CreatedAt time.Time
}

// Repository 文件存取 port（pgx 實作在 infrastructure/db）。
// Upsert 以 (meeting_id, type) 唯一，重跑覆寫同一份（冪等）。
type Repository interface {
	Upsert(ctx context.Context, meetingID uuid.UUID, t Type, content string) (Artifact, error)
	ListByMeeting(ctx context.Context, meetingID uuid.UUID) ([]Artifact, error)
}

// LLMClient 文件生成 port（Gemini 實作在 infrastructure/llm）。
type LLMClient interface {
	GeneratePRD(ctx context.Context, transcript string) (string, error)
	GenerateTechSpec(ctx context.Context, transcript string) (string, error)
}
