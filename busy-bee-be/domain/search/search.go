// Package search 語意搜尋 domain：切塊、embedding、向量檢索的 entity 與 ports（零外部依賴）。
package search

import (
	"context"

	"github.com/google/uuid"
)

const (
	MatchSemantic = "semantic"
	MatchLiteral  = "literal"
)

// Chunk 逐字稿切塊與其向量。
type Chunk struct {
	ID         uuid.UUID
	MeetingID  uuid.UUID
	UserID     uuid.UUID
	ChunkIndex int
	Content    string
	Embedding  []float32
}

// SearchResult 一筆命中會議與其最相關片段。
type SearchResult struct {
	MeetingID uuid.UUID
	Snippet   string
	Score     float64
	MatchType string
}

// Embedder 文字轉向量 port（infrastructure/llm 實作）。
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// ChunkRepository chunks 存取 port（infrastructure/db 實作）。
type ChunkRepository interface {
	// Upsert 冪等寫入單一會議的 chunks（實作先刪該會議舊 chunks 再插）。
	Upsert(ctx context.Context, chunks []Chunk) error
	DeleteByMeeting(ctx context.Context, meetingID uuid.UUID) error
	// SearchSimilar 回傳與 vec 最相近的會議（每會議取最相關片段），owner 過濾。
	SearchSimilar(ctx context.Context, userID uuid.UUID, vec []float32, topK int) ([]SearchResult, error)
	// MeetingsWithoutChunks 已 completed 但無 chunks 的會議 ID（回填掃描用）。
	MeetingsWithoutChunks(ctx context.Context) ([]uuid.UUID, error)
	// ExistingEmbeddings 回傳該會議現有 chunks 的 content → embedding 映射；
	// 重新索引時複用內容未變動的片段，省去重複 embed 呼叫（成本）。
	ExistingEmbeddings(ctx context.Context, meetingID uuid.UUID) (map[string][]float32, error)
}
