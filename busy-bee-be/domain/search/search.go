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

// RetrievedChunk RAG 問答檢索命中的片段，含所屬會議標題（供引用連結）。
// 與 SearchResult 不同：不做 per-meeting 收斂，允許同一會議多片段。
type RetrievedChunk struct {
	MeetingID  uuid.UUID
	Title      string
	ChunkIndex int
	Content    string
	Score      float64
}

// QASource 提供給 LLM 的編號來源，同時回傳前端渲染 [n] 引用連結。
type QASource struct {
	Index     int
	MeetingID uuid.UUID
	Title     string
	Content   string
}

// QAResult 跨會議 RAG 問答結果：答案（markdown，可含 [n] 引用標註）、
// 來源清單、以及是否因無相關內容而未呼叫 LLM。
type QAResult struct {
	Answer  string
	Sources []QASource
	NoMatch bool
}

// MeetingBrief QA 用的會議目錄項（標題/日期/一句話摘要），供回答「有哪些會議」、
// 「某段時間開了什麼會議」等列舉/時間型問題——答案在會議 metadata 而非逐字稿內容。
type MeetingBrief struct {
	Title   string
	Date    string // 已格式化（如 2026-07-26）
	Summary string
}

// Embedder 文字轉向量 port（infrastructure/llm 實作）。
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// QARetriever RAG 跨會議問答檢索 port（infrastructure/db 實作）。
// 回傳 top-K 相近片段（不收斂、含標題），owner 過濾。與 ChunkRepository 分離以符合 ISP。
type QARetriever interface {
	SearchSimilarForQA(ctx context.Context, userID uuid.UUID, vec []float32, topK int) ([]RetrievedChunk, error)
}

// Answerer 依問題、編號內容來源與會議清單生成帶引用的答案 port（infrastructure/llm 實作）。
// sources 供內容型問題（以 [n] 標註），meetings 供列舉/時間型問題。
type Answerer interface {
	Answer(ctx context.Context, question string, sources []QASource, meetings []MeetingBrief) (string, error)
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
