# RAG 語意搜尋 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 為 busy-bee 加入語意搜尋——逐字稿切塊後以 Gemini embedding 存 pgvector，查詢時字面（ILIKE）+ 語意（向量）混合呈現。

**Architecture:** 對齊 Clean Architecture。會議 completed 後觸發獨立索引階段（切塊→embed→存 `transcript_chunks`），既有會議由 sweeper 回填。查詢並行字面與語意，合併去重排序，查詢 embedding 失敗降級純 ILIKE。

**Tech Stack:** Go 1.26、pgx v5、pgvector（`pgvector/pgvector-go`）、Gemini `gemini-embedding-001`（768 維）、`google.golang.org/genai`。

## Global Constraints

- domain 層零外部依賴；transaction boundary 在 application 層
- 所有 handler 走 `response` package，禁止 `c.JSON`
- 業務查詢一律 `user_id` 過濾；禁止 log transcript 全文
- 包外部錯誤用 `apperr.Wrap`；外部 cause 只進 log
- 單元測試與目標同目錄 `<file>_test.go`；domain/application 用 mock port
- embedding 維度鎖定 768（換維度需重新 embed 全部）
- 模組路徑：`github.com/as130232/busy-bee/busy-bee-be`

---

## 檔案結構

| 檔案 | 責任 |
|------|------|
| `db/migrations/000006_transcript_chunks.{up,down}.sql` | pgvector extension + chunks 表 |
| `domain/search/search.go` | Chunk / SearchResult entity、Embedder / ChunkRepository ports |
| `domain/search/chunker.go` | 切塊純函數 SplitIntoChunks |
| `infrastructure/llm/gemini.go`（改） | 加 EmbedContent 實作 Embedder |
| `infrastructure/db/chunk_repo.go` | pgvector 存取 |
| `db/query/chunks.sql` | sqlc query（Upsert/Delete/Search/未索引掃描） |
| `application/search/index.go` | IndexUC：切塊+embed+upsert |
| `application/search/search.go` | SearchUC：hybrid 合併+降級 |
| `interface/http/handler/meeting/handler.go`（改） | List handler 走 SearchUC |
| `worker/indexer.go` | completed 後觸發索引 + sweeper 回填 |
| `busy-bee-fe/src/components/MeetingList.tsx`（改） | 顯示 matchSnippet |

---

## Task 1: 前置——pgvector 依賴與本地/CI image

**Files:**
- Modify: `busy-bee-be/go.mod`
- Modify: `docker-compose.yml`
- Modify: `.github/workflows/deploy.yml`

**Interfaces:**
- Produces: 本地與 CI 的 PG 具備 pgvector；Go 可 import `github.com/pgvector/pgvector-go`

- [ ] **Step 1: 加 Go 依賴**

Run: `cd busy-bee-be && PATH="$PATH:/opt/homebrew/bin" go get github.com/pgvector/pgvector-go@latest`

- [ ] **Step 2: docker-compose PG image 換 pgvector 版**

`docker-compose.yml` 的 postgres service `image:` 改為 `pgvector/pgvector:pg16`（其餘設定不動）。

- [ ] **Step 3: 重啟本地 DB 並確認 extension 可用**

Run: `docker compose up -d && docker compose exec -T db psql -U postgres -d busybee -c "CREATE EXTENSION IF NOT EXISTS vector; SELECT '[1,2,3]'::vector;"`
Expected: 回傳 `[1,2,3]`（extension 正常）

- [ ] **Step 4: CI PG service image 換 pgvector**

`.github/workflows/deploy.yml` 的 `services.postgres.image` 改為 `pgvector/pgvector:pg16`。

- [ ] **Step 5: Commit**

```bash
git add busy-bee-be/go.mod busy-bee-be/go.sum docker-compose.yml .github/workflows/deploy.yml
git commit -m "chore: 加入 pgvector 依賴與本地/CI image"
```

---

## Task 2: Migration 000006——transcript_chunks 表

**Files:**
- Create: `busy-bee-be/db/migrations/000006_transcript_chunks.up.sql`
- Create: `busy-bee-be/db/migrations/000006_transcript_chunks.down.sql`

**Interfaces:**
- Produces: `transcript_chunks(id, meeting_id, user_id, chunk_index, content, embedding vector(768), created_at)`

- [ ] **Step 1: 寫 up migration**

```sql
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE transcript_chunks (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    meeting_id  uuid NOT NULL REFERENCES meetings(id) ON DELETE CASCADE,
    user_id     uuid NOT NULL,
    chunk_index int  NOT NULL,
    content     text NOT NULL,
    embedding   vector(768) NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_chunks_embedding ON transcript_chunks USING hnsw (embedding vector_cosine_ops);
CREATE INDEX idx_chunks_meeting ON transcript_chunks (meeting_id);
```

- [ ] **Step 2: 寫 down migration**

```sql
DROP TABLE IF EXISTS transcript_chunks;
-- extension 保留（其他功能可能用到）
```

- [ ] **Step 3: 跑 migration 驗證可逆**

Run: `cd busy-bee-be && PATH="$PATH:/opt/homebrew/bin" go run ./cmd/migrate up && go run ./cmd/migrate down 1 && go run ./cmd/migrate up`
Expected: 無錯誤

- [ ] **Step 4: Commit**

```bash
git add busy-bee-be/db/migrations/000006_*
git commit -m "feat: transcript_chunks migration（pgvector）"
```

---

## Task 3: domain/search——entities 與 ports

**Files:**
- Create: `busy-bee-be/domain/search/search.go`

**Interfaces:**
- Produces:
  - `Chunk{ ID, MeetingID, UserID uuid.UUID; ChunkIndex int; Content string; Embedding []float32 }`
  - `SearchResult{ MeetingID uuid.UUID; Snippet string; Score float64; MatchType string }`（MatchType = "semantic" | "literal"）
  - `Embedder interface { Embed(ctx, text string) ([]float32, error) }`
  - `ChunkRepository interface { Upsert(ctx, []Chunk) error; DeleteByMeeting(ctx, uuid.UUID) error; SearchSimilar(ctx, userID uuid.UUID, vec []float32, topK int) ([]SearchResult, error); MeetingsWithoutChunks(ctx) ([]uuid.UUID, error) }`

- [ ] **Step 1: 寫 domain 型別（零外部依賴，只 import uuid 與 context）**

```go
// Package search 語意搜尋 domain：切塊、embedding、向量檢索的 entity 與 ports。
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
	Upsert(ctx context.Context, chunks []Chunk) error
	DeleteByMeeting(ctx context.Context, meetingID uuid.UUID) error
	// SearchSimilar 回傳與 vec 最相近的會議（每會議取最相關片段），owner 過濾。
	SearchSimilar(ctx context.Context, userID uuid.UUID, vec []float32, topK int) ([]SearchResult, error)
	// MeetingsWithoutChunks 已 completed 但無 chunks 的會議 ID（回填掃描用）。
	MeetingsWithoutChunks(ctx context.Context) ([]uuid.UUID, error)
}
```

- [ ] **Step 2: 編譯確認**

Run: `cd busy-bee-be && PATH="$PATH:/opt/homebrew/bin" go build ./domain/search/`
Expected: 無錯誤

- [ ] **Step 3: Commit**

```bash
git add busy-bee-be/domain/search/search.go
git commit -m "feat: domain/search entities 與 ports"
```

---

## Task 4: 切塊純函數 SplitIntoChunks（TDD）

**Files:**
- Create: `busy-bee-be/domain/search/chunker.go`
- Test: `busy-bee-be/domain/search/chunker_test.go`

**Interfaces:**
- Produces: `SplitIntoChunks(text string, targetChars, overlapSentences int) []string`

- [ ] **Step 1: 寫失敗測試**

```go
package search

import "testing"

func TestSplitIntoChunks_ShortTextSingleChunk(t *testing.T) {
	got := SplitIntoChunks("這是一段很短的話。", 400, 1)
	if len(got) != 1 || got[0] != "這是一段很短的話。" {
		t.Fatalf("got %#v, want single chunk", got)
	}
}

func TestSplitIntoChunks_SplitsAtSentenceBoundaryNearTarget(t *testing.T) {
	// 每句約 50 字，target 100 → 每塊約 2 句
	s := ""
	for i := 0; i < 6; i++ {
		s += "這是一個大約有五十個字左右長度的測試句子用來驗證切塊邏輯是否正確運作良好。"
	}
	got := SplitIntoChunks(s, 100, 0)
	if len(got) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(got))
	}
	for _, c := range got {
		if len([]rune(c)) > 220 { // target 100 + 一句寬容
			t.Errorf("chunk too long (%d runes): %q", len([]rune(c)), c)
		}
	}
}

func TestSplitIntoChunks_OverlapCarriesLastSentence(t *testing.T) {
	s := "第一句話結束。第二句話結束。第三句話結束。第四句話結束。"
	got := SplitIntoChunks(s, 14, 1) // 每塊約 1 句，overlap 1 句
	if len(got) < 2 {
		t.Fatalf("expected multiple chunks, got %#v", got)
	}
	// 第二塊開頭應含第一塊最後一句
	if !contains(got[1], "第一句話結束。") && !contains(got[1], "第二句話結束。") {
		t.Errorf("chunk[1]=%q should overlap previous last sentence", got[1])
	}
}

func TestSplitIntoChunks_EmptyReturnsNil(t *testing.T) {
	if got := SplitIntoChunks("   ", 400, 1); len(got) != 0 {
		t.Fatalf("empty text should return no chunks, got %#v", got)
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 2: 跑測試確認失敗**

Run: `cd busy-bee-be && PATH="$PATH:/opt/homebrew/bin" go test ./domain/search/ -run TestSplitIntoChunks -v`
Expected: FAIL（undefined: SplitIntoChunks）

- [ ] **Step 3: 實作切塊**

```go
package search

import "strings"

// SplitIntoChunks 依句子邊界把逐字稿切成約 targetChars 字的塊，
// 每塊開頭帶上一塊最後 overlapSentences 句（避免切斷語意）。空白文字回 nil。
func SplitIntoChunks(text string, targetChars, overlapSentences int) []string {
	sentences := splitSentences(text)
	if len(sentences) == 0 {
		return nil
	}

	var chunks []string
	var cur []string
	curLen := 0
	for _, s := range sentences {
		cur = append(cur, s)
		curLen += len([]rune(s))
		if curLen >= targetChars {
			chunks = append(chunks, strings.Join(cur, ""))
			// 準備下一塊：帶 overlap 句
			if overlapSentences > 0 && len(cur) >= overlapSentences {
				cur = append([]string{}, cur[len(cur)-overlapSentences:]...)
				curLen = 0
				for _, s := range cur {
					curLen += len([]rune(s))
				}
			} else {
				cur = nil
				curLen = 0
			}
		}
	}
	if len(cur) > 0 {
		last := strings.Join(cur, "")
		// 若最後殘塊與前一塊完全重疊（純 overlap），不重複加
		if len(chunks) == 0 || chunks[len(chunks)-1] != last {
			chunks = append(chunks, last)
		}
	}
	return chunks
}

// splitSentences 依中英文句末標點切句，保留標點於句尾。
func splitSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var out []string
	var b strings.Builder
	for _, r := range text {
		b.WriteRune(r)
		switch r {
		case '。', '！', '？', '.', '!', '?', '\n':
			s := strings.TrimSpace(b.String())
			if s != "" {
				out = append(out, s)
			}
			b.Reset()
		}
	}
	if s := strings.TrimSpace(b.String()); s != "" {
		out = append(out, s)
	}
	return out
}
```

- [ ] **Step 4: 跑測試確認通過**

Run: `cd busy-bee-be && PATH="$PATH:/opt/homebrew/bin" go test ./domain/search/ -run TestSplitIntoChunks -v`
Expected: PASS（全部）

- [ ] **Step 5: Commit**

```bash
git add busy-bee-be/domain/search/chunker.go busy-bee-be/domain/search/chunker_test.go
git commit -m "feat: 逐字稿切塊純函數（TDD）"
```

---

## Task 5: Gemini EmbedContent（實作 Embedder）

**Files:**
- Modify: `busy-bee-be/infrastructure/llm/gemini.go`
- Test: `busy-bee-be/infrastructure/llm/embed_test.go`（僅測純轉換 helper；真實 API 呼叫走人工驗收）

**Interfaces:**
- Consumes: `search.Embedder`
- Produces: `(*GeminiClient).Embed(ctx, text string) ([]float32, error)`

- [ ] **Step 1: 確認 genai SDK embedding 簽名**

Run: `cd busy-bee-be && PATH="$PATH:/opt/homebrew/bin" go doc google.golang.org/genai Models.EmbedContent`
Expected: 得到 `EmbedContent(ctx, model string, contents []*Content, config *EmbedContentConfig)` 類簽名。若簽名不同，以實際輸出為準調整下步程式碼。

- [ ] **Step 2: 實作 Embed**

在 `gemini.go` 加（模型名用常數，維度 768）：

```go
const embedModel = "gemini-embedding-001"
const embedDim = 768

// Embed 實作 search.Embedder：單段文字轉 768 維向量。
func (c *GeminiClient) Embed(ctx context.Context, text string) ([]float32, error) {
	dim := int32(embedDim)
	resp, err := c.client.Models.EmbedContent(ctx, embedModel,
		[]*genai.Content{genai.NewContentFromText(text, genai.RoleUser)},
		&genai.EmbedContentConfig{OutputDimensionality: &dim},
	)
	if err != nil {
		return nil, fmt.Errorf("gemini embed: %w", err)
	}
	if len(resp.Embeddings) == 0 || len(resp.Embeddings[0].Values) == 0 {
		return nil, fmt.Errorf("gemini embed: empty embedding")
	}
	return resp.Embeddings[0].Values, nil
}
```

> 若 SDK 型別名與上不同（如 `ContentEmbedding`、`Values` 欄位名），依 Step 1 的 `go doc` 結果調整；語意不變：輸入文字、輸出 `[]float32`、指定 768 維。

- [ ] **Step 3: 編譯確認**

Run: `cd busy-bee-be && PATH="$PATH:/opt/homebrew/bin" go build ./infrastructure/llm/`
Expected: 無錯誤

- [ ] **Step 4: 確認 GeminiClient 滿足 Embedder（編譯期斷言）**

在 `gemini.go` 加：`var _ search.Embedder = (*GeminiClient)(nil)`（import `domain/search`）。

Run: `go build ./infrastructure/llm/`
Expected: 無錯誤（介面吻合）

- [ ] **Step 5: Commit**

```bash
git add busy-bee-be/infrastructure/llm/gemini.go
git commit -m "feat: Gemini EmbedContent 實作 Embedder"
```

---

## Task 6: chunk_repo pgvector 存取（整合測試）

**Files:**
- Create: `busy-bee-be/db/query/chunks.sql`
- Create: `busy-bee-be/infrastructure/db/chunk_repo.go`
- Test: `busy-bee-be/infrastructure/db/chunk_repo_test.go`

**Interfaces:**
- Consumes: `search.ChunkRepository`、`search.Chunk`、`search.SearchResult`
- Produces: `NewChunkRepo(pool *pgxpool.Pool) *ChunkRepo` 實作 `search.ChunkRepository`

- [ ] **Step 1: 寫失敗整合測試**

```go
package db

import (
	"context"
	"testing"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/domain/search"
)

func mkVec(seed float32) []float32 {
	v := make([]float32, 768)
	for i := range v {
		v[i] = seed
	}
	return v
}

func TestChunkRepo_UpsertAndSearchSimilar(t *testing.T) {
	pool := testPool(t)
	u := testUser(t, pool)
	mrepo := NewMeetingRepo(pool)
	m, _ := mrepo.Create(context.Background(), domainmeeting.Meeting{
		UserID: u.ID, Title: "定價會議", Status: domainmeeting.StatusCompleted, Transcript: "談價格策略",
	})
	repo := NewChunkRepo(pool)

	err := repo.Upsert(context.Background(), []search.Chunk{
		{MeetingID: m.ID, UserID: u.ID, ChunkIndex: 0, Content: "談價格策略", Embedding: mkVec(0.1)},
		{MeetingID: m.ID, UserID: u.ID, ChunkIndex: 1, Content: "無關內容", Embedding: mkVec(0.9)},
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	res, err := repo.SearchSimilar(context.Background(), u.ID, mkVec(0.1), 5)
	if err != nil {
		t.Fatalf("SearchSimilar() error = %v", err)
	}
	if len(res) == 0 || res[0].MeetingID != m.ID {
		t.Fatalf("expected meeting %s in results, got %#v", m.ID, res)
	}
	if res[0].Snippet != "談價格策略" {
		t.Errorf("snippet = %q, want 談價格策略", res[0].Snippet)
	}

	// 非本人搜不到
	other, _ := repo.SearchSimilar(context.Background(), uuid.New(), mkVec(0.1), 5)
	if len(other) != 0 {
		t.Errorf("other user should get no results, got %d", len(other))
	}
}

func TestChunkRepo_UpsertReplacesExisting(t *testing.T) {
	pool := testPool(t)
	u := testUser(t, pool)
	m, _ := NewMeetingRepo(pool).Create(context.Background(), domainmeeting.Meeting{
		UserID: u.ID, Title: "m", Status: domainmeeting.StatusCompleted,
	})
	repo := NewChunkRepo(pool)
	_ = repo.Upsert(context.Background(), []search.Chunk{{MeetingID: m.ID, UserID: u.ID, ChunkIndex: 0, Content: "a", Embedding: mkVec(0.1)}})
	// 重跑（冪等 upsert：先刪該會議舊 chunks 再插）
	if err := repo.Upsert(context.Background(), []search.Chunk{{MeetingID: m.ID, UserID: u.ID, ChunkIndex: 0, Content: "b", Embedding: mkVec(0.2)}}); err != nil {
		t.Fatalf("re-Upsert error = %v", err)
	}
	res, _ := repo.SearchSimilar(context.Background(), u.ID, mkVec(0.2), 5)
	if len(res) != 1 || res[0].Snippet != "b" {
		t.Errorf("expected single replaced chunk 'b', got %#v", res)
	}
}

func TestChunkRepo_MeetingsWithoutChunks(t *testing.T) {
	pool := testPool(t)
	u := testUser(t, pool)
	mrepo := NewMeetingRepo(pool)
	indexed, _ := mrepo.Create(context.Background(), domainmeeting.Meeting{UserID: u.ID, Title: "i", Status: domainmeeting.StatusCompleted, Transcript: "x"})
	missing, _ := mrepo.Create(context.Background(), domainmeeting.Meeting{UserID: u.ID, Title: "m", Status: domainmeeting.StatusCompleted, Transcript: "y"})
	repo := NewChunkRepo(pool)
	_ = repo.Upsert(context.Background(), []search.Chunk{{MeetingID: indexed.ID, UserID: u.ID, ChunkIndex: 0, Content: "x", Embedding: mkVec(0.1)}})

	ids, err := repo.MeetingsWithoutChunks(context.Background())
	if err != nil {
		t.Fatalf("MeetingsWithoutChunks() error = %v", err)
	}
	found := false
	for _, id := range ids {
		if id == missing.ID {
			found = true
		}
		if id == indexed.ID {
			t.Error("indexed meeting should not appear")
		}
	}
	if !found {
		t.Errorf("missing meeting %s should appear in results", missing.ID)
	}
}
```

- [ ] **Step 2: 跑測試確認失敗**

Run: `cd busy-bee-be && PATH="$PATH:/opt/homebrew/bin" go test ./infrastructure/db/ -run TestChunkRepo -v`
Expected: FAIL（undefined: NewChunkRepo）

- [ ] **Step 3: 實作 chunk_repo（手寫 pgx，用 pgvector-go 的 Vector 型別）**

```go
package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgv "github.com/pgvector/pgvector-go"

	"github.com/as130232/busy-bee/busy-bee-be/domain/search"
)

type ChunkRepo struct {
	pool *pgxpool.Pool
}

func NewChunkRepo(pool *pgxpool.Pool) *ChunkRepo { return &ChunkRepo{pool: pool} }

var _ search.ChunkRepository = (*ChunkRepo)(nil)

// Upsert 冪等：先刪該會議舊 chunks 再批次插入。
func (r *ChunkRepo) Upsert(ctx context.Context, chunks []search.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}
	meetingID := chunks[0].MeetingID
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM transcript_chunks WHERE meeting_id = $1`, meetingID); err != nil {
			return fmt.Errorf("db.chunk delete: %w", err)
		}
		for _, c := range chunks {
			_, err := tx.Exec(ctx,
				`INSERT INTO transcript_chunks (meeting_id, user_id, chunk_index, content, embedding)
				 VALUES ($1,$2,$3,$4,$5)`,
				c.MeetingID, c.UserID, c.ChunkIndex, c.Content, pgv.NewVector(c.Embedding))
			if err != nil {
				return fmt.Errorf("db.chunk insert: %w", err)
			}
		}
		return nil
	})
}

func (r *ChunkRepo) DeleteByMeeting(ctx context.Context, meetingID uuid.UUID) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM transcript_chunks WHERE meeting_id = $1`, meetingID); err != nil {
		return fmt.Errorf("db.chunk deleteByMeeting: %w", err)
	}
	return nil
}

// SearchSimilar 每會議取最近的一塊（DISTINCT ON），cosine 距離排序，owner 過濾。
func (r *ChunkRepo) SearchSimilar(ctx context.Context, userID uuid.UUID, vec []float32, topK int) ([]search.SearchResult, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT meeting_id, content, 1 - (embedding <=> $2) AS score
		FROM (
			SELECT DISTINCT ON (meeting_id) meeting_id, content, embedding
			FROM transcript_chunks
			WHERE user_id = $1
			ORDER BY meeting_id, embedding <=> $2
		) t
		ORDER BY embedding <=> $2
		LIMIT $3`,
		userID, pgv.NewVector(vec), topK)
	if err != nil {
		return nil, fmt.Errorf("db.chunk search: %w", err)
	}
	defer rows.Close()
	var out []search.SearchResult
	for rows.Next() {
		var res search.SearchResult
		if err := rows.Scan(&res.MeetingID, &res.Snippet, &res.Score); err != nil {
			return nil, fmt.Errorf("db.chunk scan: %w", err)
		}
		res.MatchType = search.MatchSemantic
		out = append(out, res)
	}
	return out, rows.Err()
}

func (r *ChunkRepo) MeetingsWithoutChunks(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.id FROM meetings m
		WHERE m.status = 'completed' AND m.transcript <> ''
		  AND NOT EXISTS (SELECT 1 FROM transcript_chunks c WHERE c.meeting_id = m.id)`)
	if err != nil {
		return nil, fmt.Errorf("db.chunk missing: %w", err)
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("db.chunk missing scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
```

> `db/query/chunks.sql` 本 task 不用 sqlc（pgvector 型別 sqlc 支援不完整），改手寫 pgx。故不需 `chunks.sql`——從檔案結構移除，此為刻意決定。

- [ ] **Step 4: 跑測試確認通過**

Run: `cd busy-bee-be && PATH="$PATH:/opt/homebrew/bin" go test ./infrastructure/db/ -run TestChunkRepo -v`
Expected: PASS（三個都過）

- [ ] **Step 5: Commit**

```bash
git add busy-bee-be/infrastructure/db/chunk_repo.go busy-bee-be/infrastructure/db/chunk_repo_test.go
git commit -m "feat: chunk_repo pgvector 存取（整合測試）"
```

---

## Task 7: IndexUC——切塊+embed+存（TDD mock）

**Files:**
- Create: `busy-bee-be/application/search/index.go`
- Test: `busy-bee-be/application/search/index_test.go`

**Interfaces:**
- Consumes: `search.Embedder`、`search.ChunkRepository`、`domainmeeting.Repository`（取 transcript）
- Produces: `NewIndexUC(meetings domainmeeting.Repository, embedder search.Embedder, chunks search.ChunkRepository) *IndexUC`；`(*IndexUC).Execute(ctx, meetingID uuid.UUID) error`

- [ ] **Step 1: 寫失敗測試**

```go
package search

import (
	"context"
	"testing"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainsearch "github.com/as130232/busy-bee/busy-bee-be/domain/search"
)

type fakeMeetingRepo struct{ m domainmeeting.Meeting }

func (f *fakeMeetingRepo) Get(_ context.Context, id uuid.UUID) (domainmeeting.Meeting, error) {
	return f.m, nil
}

type fakeEmbedder struct{ calls int }

func (f *fakeEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	f.calls++
	return []float32{0.1, 0.2}, nil
}

type fakeChunkRepo struct {
	upserted []domainsearch.Chunk
	existing bool
}

func (f *fakeChunkRepo) Upsert(_ context.Context, cs []domainsearch.Chunk) error {
	f.upserted = cs
	return nil
}
func (f *fakeChunkRepo) DeleteByMeeting(context.Context, uuid.UUID) error { return nil }
func (f *fakeChunkRepo) SearchSimilar(context.Context, uuid.UUID, []float32, int) ([]domainsearch.SearchResult, error) {
	return nil, nil
}
func (f *fakeChunkRepo) MeetingsWithoutChunks(context.Context) ([]uuid.UUID, error) {
	if f.existing {
		return nil, nil
	}
	return nil, nil
}

func TestIndexUC_ChunksEmbedAndUpsert(t *testing.T) {
	mid := uuid.New()
	uid := uuid.New()
	mrepo := &fakeMeetingRepo{m: domainmeeting.Meeting{ID: mid, UserID: uid,
		Status: domainmeeting.StatusCompleted, Transcript: "第一句很長的內容。第二句也很長的內容。"}}
	emb := &fakeEmbedder{}
	crepo := &fakeChunkRepo{}
	uc := NewIndexUC(mrepo, emb, crepo)

	if err := uc.Execute(context.Background(), mid); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(crepo.upserted) == 0 {
		t.Fatal("expected chunks upserted")
	}
	if emb.calls != len(crepo.upserted) {
		t.Errorf("embed calls %d != chunks %d", emb.calls, len(crepo.upserted))
	}
	if crepo.upserted[0].UserID != uid || crepo.upserted[0].MeetingID != mid {
		t.Error("chunk missing owner/meeting id")
	}
}

func TestIndexUC_EmptyTranscriptSkips(t *testing.T) {
	mid := uuid.New()
	mrepo := &fakeMeetingRepo{m: domainmeeting.Meeting{ID: mid, Status: domainmeeting.StatusCompleted, Transcript: ""}}
	emb := &fakeEmbedder{}
	crepo := &fakeChunkRepo{}
	uc := NewIndexUC(mrepo, emb, crepo)

	if err := uc.Execute(context.Background(), mid); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if emb.calls != 0 || len(crepo.upserted) != 0 {
		t.Error("empty transcript should skip embedding/upsert")
	}
}
```

- [ ] **Step 2: 跑測試確認失敗**

Run: `cd busy-bee-be && PATH="$PATH:/opt/homebrew/bin" go test ./application/search/ -run TestIndexUC -v`
Expected: FAIL（undefined: NewIndexUC）

- [ ] **Step 3: 實作 IndexUC**

```go
// Package search application：索引（IndexUC）與查詢（SearchUC）use cases。
package search

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainsearch "github.com/as130232/busy-bee/busy-bee-be/domain/search"
)

const (
	chunkTargetChars = 400
	chunkOverlap     = 1
)

// meetingGetter 窄介面：只取單一會議（IndexUC 所需）。
type meetingGetter interface {
	Get(ctx context.Context, id uuid.UUID) (domainmeeting.Meeting, error)
}

type IndexUC struct {
	meetings meetingGetter
	embedder domainsearch.Embedder
	chunks   domainsearch.ChunkRepository
}

func NewIndexUC(meetings meetingGetter, embedder domainsearch.Embedder, chunks domainsearch.ChunkRepository) *IndexUC {
	return &IndexUC{meetings: meetings, embedder: embedder, chunks: chunks}
}

// Execute 切塊 → 逐塊 embed → upsert（冪等：Upsert 內部先刪後插）。空逐字稿跳過。
func (uc *IndexUC) Execute(ctx context.Context, meetingID uuid.UUID) error {
	m, err := uc.meetings.Get(ctx, meetingID)
	if err != nil {
		return fmt.Errorf("index get meeting: %w", err)
	}
	parts := domainsearch.SplitIntoChunks(m.Transcript, chunkTargetChars, chunkOverlap)
	if len(parts) == 0 {
		return nil
	}
	chunks := make([]domainsearch.Chunk, 0, len(parts))
	for i, p := range parts {
		vec, err := uc.embedder.Embed(ctx, p)
		if err != nil {
			return fmt.Errorf("index embed chunk %d: %w", i, err)
		}
		chunks = append(chunks, domainsearch.Chunk{
			MeetingID: m.ID, UserID: m.UserID, ChunkIndex: i, Content: p, Embedding: vec,
		})
	}
	if err := uc.chunks.Upsert(ctx, chunks); err != nil {
		return fmt.Errorf("index upsert: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: 跑測試確認通過**

Run: `cd busy-bee-be && PATH="$PATH:/opt/homebrew/bin" go test ./application/search/ -run TestIndexUC -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add busy-bee-be/application/search/index.go busy-bee-be/application/search/index_test.go
git commit -m "feat: IndexUC 切塊+embed+存（TDD）"
```

---

## Task 8: SearchUC——hybrid 合併+降級（TDD mock）

**Files:**
- Create: `busy-bee-be/application/search/search.go`
- Test: `busy-bee-be/application/search/search_test.go`

**Interfaces:**
- Consumes: `search.Embedder`、`search.ChunkRepository`、字面搜尋來源（`domainmeeting.Repository.ListForUser`）
- Produces: `NewSearchUC(literal literalSearcher, embedder search.Embedder, chunks search.ChunkRepository) *SearchUC`；`(*SearchUC).Execute(ctx, userID uuid.UUID, query string) ([]domainmeeting.Meeting, map[uuid.UUID]domainsearch.SearchResult, error)`（回傳排序後會議 + 每會議命中片段）

- [ ] **Step 1: 寫失敗測試**

```go
package search

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainsearch "github.com/as130232/busy-bee/busy-bee-be/domain/search"
)

type fakeLiteral struct {
	meetings []domainmeeting.Meeting
	err      error
}

func (f *fakeLiteral) ListForUser(_ context.Context, _ uuid.UUID, _ string) ([]domainmeeting.Meeting, error) {
	return f.meetings, f.err
}

type searchFakeChunks struct {
	results []domainsearch.SearchResult
}

func (f *searchFakeChunks) Upsert(context.Context, []domainsearch.Chunk) error { return nil }
func (f *searchFakeChunks) DeleteByMeeting(context.Context, uuid.UUID) error   { return nil }
func (f *searchFakeChunks) SearchSimilar(context.Context, uuid.UUID, []float32, int) ([]domainsearch.SearchResult, error) {
	return f.results, nil
}
func (f *searchFakeChunks) MeetingsWithoutChunks(context.Context) ([]uuid.UUID, error) {
	return nil, nil
}

func TestSearchUC_MergesSemanticAndLiteral(t *testing.T) {
	semID := uuid.New()
	litID := uuid.New()
	lit := &fakeLiteral{meetings: []domainmeeting.Meeting{{ID: litID, Title: "字面命中"}}}
	chunks := &searchFakeChunks{results: []domainsearch.SearchResult{{MeetingID: semID, Snippet: "語意片段", Score: 0.9, MatchType: domainsearch.MatchSemantic}}}
	uc := NewSearchUC(lit, &fakeEmbedder{}, chunks)

	meetings, hits, err := uc.Execute(context.Background(), uuid.New(), "查詢")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	ids := map[uuid.UUID]bool{}
	for _, m := range meetings {
		ids[m.ID] = true
	}
	if !ids[semID] || !ids[litID] {
		t.Errorf("expected both semantic %s and literal %s, got %#v", semID, litID, ids)
	}
	if hits[semID].Snippet != "語意片段" {
		t.Errorf("semantic hit snippet missing: %#v", hits[semID])
	}
}

func TestSearchUC_EmbedFailsFallsBackToLiteral(t *testing.T) {
	litID := uuid.New()
	lit := &fakeLiteral{meetings: []domainmeeting.Meeting{{ID: litID, Title: "字面"}}}
	uc := NewSearchUC(lit, &failEmbedder{}, &searchFakeChunks{})

	meetings, _, err := uc.Execute(context.Background(), uuid.New(), "查詢")
	if err != nil {
		t.Fatalf("Execute() should not error on embed failure, got %v", err)
	}
	if len(meetings) != 1 || meetings[0].ID != litID {
		t.Errorf("expected literal fallback result, got %#v", meetings)
	}
}

type failEmbedder struct{}

func (f *failEmbedder) Embed(context.Context, string) ([]float32, error) {
	return nil, errors.New("embed down")
}
```

- [ ] **Step 2: 跑測試確認失敗**

Run: `cd busy-bee-be && PATH="$PATH:/opt/homebrew/bin" go test ./application/search/ -run TestSearchUC -v`
Expected: FAIL（undefined: NewSearchUC）

- [ ] **Step 3: 實作 SearchUC**

```go
package search

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainsearch "github.com/as130232/busy-bee/busy-bee-be/domain/search"
)

const searchTopK = 10
const literalBoost = 0.5 // 字面命中的固定分（與 cosine 0~1 同量級）

// literalSearcher 字面搜尋來源（MeetingRepo.ListForUser 滿足）。
type literalSearcher interface {
	ListForUser(ctx context.Context, userID uuid.UUID, search string) ([]domainmeeting.Meeting, error)
}

type SearchUC struct {
	literal  literalSearcher
	embedder domainsearch.Embedder
	chunks   domainsearch.ChunkRepository
}

func NewSearchUC(literal literalSearcher, embedder domainsearch.Embedder, chunks domainsearch.ChunkRepository) *SearchUC {
	return &SearchUC{literal: literal, embedder: embedder, chunks: chunks}
}

// Execute 並行字面 + 語意，合併去重排序。查詢 embedding 失敗降級純字面。
// 回傳排序後會議，與每會議命中片段（semantic 才有 snippet）。
func (uc *SearchUC) Execute(ctx context.Context, userID uuid.UUID, query string) ([]domainmeeting.Meeting, map[uuid.UUID]domainsearch.SearchResult, error) {
	litMeetings, err := uc.literal.ListForUser(ctx, userID, query)
	if err != nil {
		return nil, nil, fmt.Errorf("search literal: %w", err)
	}

	scores := map[uuid.UUID]float64{}
	hits := map[uuid.UUID]domainsearch.SearchResult{}
	byID := map[uuid.UUID]domainmeeting.Meeting{}
	for _, m := range litMeetings {
		byID[m.ID] = m
		scores[m.ID] += literalBoost
	}

	// 語意：embedding 失敗則降級（只用字面結果）
	if vec, eerr := uc.embedder.Embed(ctx, query); eerr != nil {
		slog.WarnContext(ctx, "search.semantic_degraded", "err", eerr)
	} else if sem, serr := uc.chunks.SearchSimilar(ctx, userID, vec, searchTopK); serr != nil {
		slog.WarnContext(ctx, "search.semantic_degraded", "err", serr)
	} else {
		for _, r := range sem {
			scores[r.MeetingID] += r.Score
			hits[r.MeetingID] = r // semantic snippet 優先
		}
	}

	// 補齊只在語意命中、不在字面清單的會議
	var missing []uuid.UUID
	for id := range scores {
		if _, ok := byID[id]; !ok {
			missing = append(missing, id)
		}
	}
	for _, id := range missing {
		if m, gerr := uc.getMeeting(ctx, userID, id); gerr == nil {
			byID[id] = m
		} else {
			delete(scores, id) // 取不到（理論上不會，owner 已過濾）就丟棄
		}
	}

	// 依分數排序
	ids := make([]uuid.UUID, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return scores[ids[i]] > scores[ids[j]] })

	out := make([]domainmeeting.Meeting, 0, len(ids))
	for _, id := range ids {
		out = append(out, byID[id])
	}
	return out, hits, nil
}
```

> `getMeeting` 需要 owner 過濾取單一會議。SearchUC 增一個依賴 `meetingByID` 窄介面：`GetForUser(ctx, id, userID) (domainmeeting.Meeting, error)`。在 `NewSearchUC` 簽名補一個參數 `owner meetingByID`，並在測試 fake 補此方法。實作時把 `literalSearcher` 與 `meetingByID` 都由 `*db.MeetingRepo` 滿足（wiring 時傳同一個）。

- [ ] **Step 4: 依上註補齊 getMeeting 依賴後跑測試**

Run: `cd busy-bee-be && PATH="$PATH:/opt/homebrew/bin" go test ./application/search/ -run TestSearchUC -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add busy-bee-be/application/search/search.go busy-bee-be/application/search/search_test.go
git commit -m "feat: SearchUC hybrid 合併+降級（TDD）"
```

---

## Task 9: worker 索引觸發 + sweeper 回填

**Files:**
- Create: `busy-bee-be/worker/indexer.go`
- Modify: `busy-bee-be/application/meeting/process.go`（completed 後觸發索引）
- Test: `busy-bee-be/worker/indexer_test.go`

**Interfaces:**
- Consumes: `*search.IndexUC`、`search.ChunkRepository.MeetingsWithoutChunks`
- Produces: `RunIndexBackfill(ctx, chunks search.ChunkRepository, index *appsearch.IndexUC, interval time.Duration)`

- [ ] **Step 1: 寫失敗測試（回填掃描把未索引會議交給 IndexUC）**

```go
package worker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeChunkScanner struct{ ids []uuid.UUID }

func (f *fakeChunkScanner) MeetingsWithoutChunks(context.Context) ([]uuid.UUID, error) {
	return f.ids, nil
}

type recordingIndexer struct{ indexed []uuid.UUID }

func (r *recordingIndexer) Execute(_ context.Context, id uuid.UUID) error {
	r.indexed = append(r.indexed, id)
	return nil
}

func TestBackfillIndexesMissingMeetings(t *testing.T) {
	want := uuid.New()
	scanner := &fakeChunkScanner{ids: []uuid.UUID{want}}
	idx := &recordingIndexer{}

	backfillOnce(context.Background(), scanner, idx)

	if len(idx.indexed) != 1 || idx.indexed[0] != want {
		t.Fatalf("expected backfill to index %s, got %#v", want, idx.indexed)
	}
	_ = time.Second
}
```

- [ ] **Step 2: 跑測試確認失敗**

Run: `cd busy-bee-be && PATH="$PATH:/opt/homebrew/bin" go test ./worker/ -run TestBackfill -v`
Expected: FAIL（undefined: backfillOnce）

- [ ] **Step 3: 實作 indexer.go**

```go
package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/as130232/busy-bee/busy-bee-be/domain/search"
)

// meetingIndexer 窄介面（*application/search.IndexUC 滿足）。
type meetingIndexer interface {
	Execute(ctx context.Context, meetingID uuid.UUID) error
}

type chunkScanner interface {
	MeetingsWithoutChunks(ctx context.Context) ([]uuid.UUID, error)
}

func backfillOnce(ctx context.Context, scanner chunkScanner, index meetingIndexer) {
	ids, err := scanner.MeetingsWithoutChunks(ctx)
	if err != nil {
		slog.WarnContext(ctx, "index.backfill.scan_failed", "err", err)
		return
	}
	for _, id := range ids {
		if err := index.Execute(ctx, id); err != nil {
			slog.WarnContext(ctx, "index.backfill.index_failed", "meeting_id", id, "err", err)
		}
	}
}

// RunIndexBackfill 啟動掃一次，之後每 interval 掃未索引會議補索引。
func RunIndexBackfill(ctx context.Context, scanner search.ChunkRepository, index meetingIndexer, interval time.Duration) {
	backfillOnce(ctx, scanner, index)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			backfillOnce(ctx, scanner, index)
		}
	}
}
```

- [ ] **Step 4: process.go completed 後觸發索引（非阻塞，失敗不回退 completed）**

在 `ProcessUC` 加一個可選的 `indexer meetingIndexer` 欄位（nil 時跳過），於 `SetCompleted` 成功後呼叫。若不改 ProcessUC 建構子，改為在 `Execute` 回傳前：`if uc.indexer != nil { if err := uc.indexer.Execute(ctx, m.ID); err != nil { slog.WarnContext(ctx, "index.after_complete_failed", "meeting_id", m.ID, "err", err) } }`。此為 best-effort，真正保底由回填掃描負責。

- [ ] **Step 5: 跑測試確認通過**

Run: `cd busy-bee-be && PATH="$PATH:/opt/homebrew/bin" go test ./worker/ -run TestBackfill -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add busy-bee-be/worker/indexer.go busy-bee-be/worker/indexer_test.go busy-bee-be/application/meeting/process.go
git commit -m "feat: 索引回填掃描 + completed 後觸發索引"
```

---

## Task 10: HTTP List handler 走 SearchUC + openapi

**Files:**
- Modify: `busy-bee-be/interface/http/handler/meeting/handler.go`（List）
- Modify: `busy-bee-be/interface/http/handler/meeting/response.go`（加 matchSnippet/matchType）
- Modify: `busy-bee-be/api/openapi.yaml`（Meeting schema 加選填欄位）
- Test: `busy-bee-be/interface/http/handler/meeting/handler_test.go`（search 回傳 snippet）

**Interfaces:**
- Consumes: `*appsearch.SearchUC`
- Produces: `GET /meetings?search=` 回傳每筆 `matchSnippet`/`matchType`

- [ ] **Step 1: 寫失敗測試（search 有值時回傳 snippet）**

```go
func TestList_WithSearchReturnsSnippet(t *testing.T) {
	e := testRouter(t) // 需在 testRouter 注入 Search UC，見 Step 3
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/meetings?search=定價", nil)
	e.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "matchSnippet") {
		t.Errorf("expected matchSnippet in body: %s", w.Body.String())
	}
}
```

- [ ] **Step 2: 跑測試確認失敗**

Run: `cd busy-bee-be && PATH="$PATH:/opt/homebrew/bin" go test ./interface/http/handler/meeting/ -run TestList_WithSearch -v`
Expected: FAIL

- [ ] **Step 3: 改 List handler——search 非空走 SearchUC，回傳 snippet**

`meetingResponse` 加：`MatchSnippet string \`json:"matchSnippet,omitempty"\`` 與 `MatchType string \`json:"matchType,omitempty"\``。
`HandlerUCs` 加 `Search *appsearch.SearchUC`。List handler：

```go
func (h *Handler) List(c *gin.Context) {
	userID, ok := domainuser.IDFrom(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.New(errcode.Unauthorized))
		return
	}
	search := strings.TrimSpace(c.Query("search"))
	if search == "" || h.uc.Search == nil {
		list, err := h.uc.List.Execute(c.Request.Context(), userID, search)
		if err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, gin.H{"meetings": toMeetingResponses(list, nil)})
		return
	}
	meetings, hits, err := h.uc.Search.Execute(c.Request.Context(), userID, search)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"meetings": toMeetingResponses(meetings, hits)})
}
```

加 helper `toMeetingResponses(list []domainmeeting.Meeting, hits map[uuid.UUID]domainsearch.SearchResult) []meetingResponse`，對 hits 內的會議填 snippet/type。

- [ ] **Step 4: openapi Meeting schema 加選填欄位 + 重生 TS client**

`api/openapi.yaml` 的 `Meeting` schema `properties` 加 `matchSnippet: { type: string }`、`matchType: { type: string }`。
Run: `cd busy-bee-fe && npm run gen:api`

- [ ] **Step 5: wiring——main.go 組 SearchUC 注入 handler**

`cmd/server/main.go`：建 `chunkRepo := db.NewChunkRepo(pool)`、`indexUC := appsearch.NewIndexUC(meetingRepo, gemini, chunkRepo)`、`searchUC := appsearch.NewSearchUC(meetingRepo, gemini, chunkRepo, meetingRepo)`（literal 與 owner 都用 meetingRepo）；`HandlerUCs.Search = searchUC`；`go worker.RunIndexBackfill(sweepCtx, chunkRepo, indexUC, 5*time.Minute)`；ProcessUC 注入 indexUC。

- [ ] **Step 6: 跑全測試**

Run: `cd busy-bee-be && PATH="$PATH:/opt/homebrew/bin" go vet ./... && go test ./...`
Expected: 全 PASS

- [ ] **Step 7: Commit**

```bash
git add busy-bee-be/ busy-bee-fe/src/services/api/schema.d.ts
git commit -m "feat: List handler hybrid 搜尋 + snippet + wiring"
```

---

## Task 11: 前端顯示 matchSnippet

**Files:**
- Modify: `busy-bee-fe/src/components/MeetingList.tsx`

**Interfaces:**
- Consumes: `Meeting.matchSnippet`（openapi 生成型別）

- [ ] **Step 1: MeetingList 顯示 snippet（有 matchSnippet 時）**

在列表項 `subtitle` 下方，若 `m.matchSnippet` 存在，渲染一行片段（query 詞可先不高亮，純顯示）：

```tsx
{m.matchSnippet && (
  <span className="mt-1 block truncate text-xs text-muted italic">
    …{m.matchSnippet}…
  </span>
)}
```

- [ ] **Step 2: typecheck + build**

Run: `cd busy-bee-fe && npx tsc -b --noEmit && npm run build`
Expected: 無錯誤

- [ ] **Step 3: Commit**

```bash
git add busy-bee-fe/src/components/MeetingList.tsx
git commit -m "feat: 前端搜尋結果顯示逐字稿片段"
```

---

## Task 12: 部署前置與人工驗收

**Files:**
- Modify: `docs/PLAN.md`（Phase 15 標 ✅、Session Log）

- [ ] **Step 1: Neon 啟用 pgvector（migration 已含 CREATE EXTENSION，CI 自動跑；若手動）**

Run（僅在 CI 未涵蓋時）：Neon SQL editor 執行 `CREATE EXTENSION IF NOT EXISTS vector;`

- [ ] **Step 2: 本地 e2e 人工驗收**

啟動本地後端 + 前端，錄一段提到「價格策略」的會議，處理完成後在搜尋框輸入「定價」，確認能搜到該會議且顯示片段。

- [ ] **Step 3: 更新 PLAN.md Phase 15 狀態、Session Log**

- [ ] **Step 4: finishing-a-development-branch**

驗證測試全綠 → 呈現 4 選項 → merge/PR。

---

## Self-Review

**Spec coverage：**
- 索引（資料流 A）→ Task 7 IndexUC + Task 9 completed 觸發
- 回填（資料流 B）→ Task 9 RunIndexBackfill
- 搜尋（資料流 C）→ Task 8 SearchUC + Task 10 handler
- 資料模型 → Task 2 migration
- domain ports → Task 3；chunker → Task 4；Embedder → Task 5；pgvector repo → Task 6
- API + snippet → Task 10；前端 → Task 11
- 降級 → Task 8（TestSearchUC_EmbedFallsBack）
- 冪等 → Task 6（UpsertReplaces）+ Task 7
- owner 過濾 → Task 6（非本人搜不到）
- 刪除 cascade → Task 2（ON DELETE CASCADE）
- 成本/前置 → Task 1、Task 12

**Placeholder scan：** 無 TBD；Task 5 embedding 與 Task 8 getMeeting 依賴有明確「依 go doc / 補窄介面」指示（非 placeholder，是實作時的具體動作）。

**Type consistency：** `search.Chunk`/`SearchResult`/`Embedder`/`ChunkRepository` 全計畫一致；`Embed(ctx,text)([]float32,error)`、`SearchSimilar(userID,vec,topK)` 各處吻合。

**已知需實作時確認：** (1) genai `EmbedContent` 確切型別（Task 5 Step 1 go doc）；(2) SearchUC 的 owner 取單筆會議依賴（Task 8 Step 3 註記，NewSearchUC 補第 4 參數）。兩者已在對應 task 標明具體處理方式。
