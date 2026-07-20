# RAG 語意搜尋設計（Phase 15）

> 狀態：設計定稿，待實作
> 日期：2026-07-20
> 對應 ADR-006 升級路徑（pgvector + Gemini embedding）

## 1. 背景與目標

現有搜尋是 `ILIKE '%keyword%'` 搜 `meetings.title` + `transcript`（ADR-006），純字面比對——搜「定價」找不到「價格策略」。本功能新增**語意搜尋**：以向量相似度找出語意相關的會議與逐字稿片段，與既有字面搜尋**混合**（hybrid）呈現。

**核心目標**：使用者在既有搜尋框輸入自然語言，得到語意相關的會議清單，每筆附最相關的逐字稿片段。

## 2. 範圍

### 目標（本 spec）
- 逐字稿切塊 → Gemini embedding → 存 `transcript_chunks`（pgvector）
- 既有已完成會議的一次性回填索引
- 查詢：字面（ILIKE）+ 語意（向量 top-K）混合，同一個搜尋框、合併結果
- 回傳每筆會議的最相關片段 snippet

### 非目標（留待後續）
- **RAG 問答**（檢索後餵 LLM 生成答案）：本 spec 只做語意「搜尋」，問答為後續擴充，但索引基建（切塊 + embedding + 向量檢索）為其鋪路
- embedding 生成的內容源僅限**逐字稿**，不含 PRD / TechSpec / 行動項（衍生摘要，語意已被逐字稿涵蓋）
- 說話者分離、跨使用者共享搜尋（維持 owner-only）

## 3. 架構與資料流

依賴方向對齊 Clean Architecture（interface / worker → application → domain ← infrastructure）。

### 資料流 A：索引（寫入）
處理 pipeline 現況：`STT → 生成文件/行動項 → completed`。**會議進入 completed 後，觸發獨立的索引階段**（與 completed 狀態解耦——索引失敗不回退 completed，會議仍算處理完成）：
1. 取會議 `transcript`
2. 切塊（chunker 純函數）
3. 每塊呼叫 Gemini embedding
4. upsert 進 `transcript_chunks`

**冪等**（對齊 ADR-009）：會議已有 chunks 則跳過，retry / 重啟安全。索引失敗不影響已生成的文件，由回填掃描（資料流 B）下次重試。

### 資料流 B：回填（既有資料）
production 已有 completed 會議但無 chunks。啟動時由 sweeper 掃「completed 且無 chunks」的會議，排入索引佇列（複用現有 in-memory queue + sweeper 模式，ADR-010）。無需手動操作。

### 資料流 C：搜尋（查詢）
使用者在既有搜尋框輸入 query，後端並行：
1. **字面**：`ILIKE` 搜 title + transcript（現有邏輯）
2. **語意**：query 算 embedding → pgvector cosine 找 top-K chunks → 映射回會議

合併去重、排序後回傳會議清單，每筆帶最相關的逐字稿片段 snippet。**查詢時 embedding 失敗 → 降級為純 ILIKE**（搜尋不整個壞掉）。

## 4. 資料模型

新增 migration `000006_transcript_chunks`（不動現有表）：

```sql
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE transcript_chunks (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    meeting_id  uuid NOT NULL REFERENCES meetings(id) ON DELETE CASCADE,
    user_id     uuid NOT NULL,              -- 冗餘存，owner 過濾免 join
    chunk_index int  NOT NULL,
    content     text NOT NULL,              -- 該塊原文，回傳 snippet 用
    embedding   vector(768) NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ON transcript_chunks USING hnsw (embedding vector_cosine_ops);
CREATE INDEX ON transcript_chunks (meeting_id);
```

- `ON DELETE CASCADE`：會議刪除時自動清 chunks
- `user_id` 冗餘：向量查詢直接 `WHERE user_id = $1`，避免 join meetings（資料安全規範：業務查詢一律 owner 過濾）
- 資料量小時 HNSW 索引非必要（brute-force 夠快），但預留

## 5. 分層設計

### domain/search/
- `Chunk` entity：`{ ID, MeetingID, UserID, ChunkIndex, Content, Embedding []float32 }`
- `SearchResult` entity：`{ MeetingID, Snippet, Score, MatchType }`（MatchType = semantic | literal）
- `Embedder` port：`Embed(ctx, text string) ([]float32, error)`
- `ChunkRepository` port：
  - `Upsert(ctx, chunks []Chunk) error`
  - `DeleteByMeeting(ctx, meetingID) error`
  - `SearchSimilar(ctx, userID uuid.UUID, vec []float32, topK int) ([]SearchResult, error)`
  - `MeetingsWithoutChunks(ctx) ([]uuid.UUID, error)`（回填掃描用）

### infrastructure/llm/
- `GeminiClient` 新增 `EmbedContent(ctx, text) ([]float32, error)`，實作 `search.Embedder`。用 `genai` SDK 的 embedding 端點，模型 `gemini-embedding-001`，輸出維度 768。

### infrastructure/db/
- `chunk_repo.go`：pgvector 存取，用 `github.com/pgvector/pgvector-go`（pgx v5 相容）。cosine 距離運算子 `<=>`。

### application/meeting/（或 application/search/）
- `IndexUC`：`Execute(ctx, meetingID)`——取 transcript → chunker → 逐塊 embed → `Upsert`。冪等（已有 chunks 跳過）。
- `SearchUC`：`Execute(ctx, userID, query) ([]SearchResult, error)`——並行字面 + 語意，合併去重排序。降級處理。

### interface/http/
- 升級 `GET /api/v1/meetings?search=` 走 `SearchUC`（search 非空時）；回傳每筆帶 `matchSnippet`。

### worker/
- pipeline 完成後觸發 `IndexUC`（新增階段或 completed 後 hook）
- sweeper 週期掃 `MeetingsWithoutChunks` 補回填

## 6. 技術選擇

| 決策點 | 選擇 | 理由 |
|--------|------|------|
| Embedding 模型 | Gemini `gemini-embedding-001`，輸出 768 維 | 品質佳、維度可裁切省空間；與現有 Gemini key 同計費 |
| 向量距離 | cosine（`vector_cosine_ops` / `<=>`） | 語意相似度標準做法 |
| 索引 | HNSW | Neon 原生支援；資料量小時可省，預留 |
| 切塊 | 按句子邊界累積 ~400 字成塊，塊間 1 句 overlap | Whisper segments 太碎；overlap 避免切斷語意 |
| Hybrid 合併 | 語意 top-K 為主，字面命中的會議提權去重 | 資料量小，先簡單；不上 RRF（YAGNI） |
| top-K | 預設 10（可調） | side project 資料量足夠 |

### 切塊細節
`chunker(text string, targetChars int, overlapSentences int) []string`：
- 按中英文句子邊界（。！？.!? + 換行）切句
- 累積句子到 ~400 字成一塊
- 每塊開頭帶上一塊最後 1 句（overlap）
- 空 / 極短 transcript → 回單塊或空
- 純函數，無外部依賴，完整 TDD

### Hybrid 合併細節
- 字面命中的會議：加固定提權分
- 語意命中的會議：用 cosine 相似度分
- 同會議兩邊都命中 → 取較高分、標 MatchType（優先顯示 semantic snippet）
- 依分數排序，回傳去重會議清單

## 7. API 契約

`GET /api/v1/meetings?search=<query>`（search 為空時維持現有列表行為）

回傳：
```json
{
  "meetings": [
    {
      "id": "...", "title": "...", "status": "...",
      "durationSeconds": 0, "createdAt": "...",
      "matchSnippet": "...最相關的逐字稿片段...",
      "matchType": "semantic"
    }
  ]
}
```
- `matchSnippet` / `matchType`：僅 search 非空時出現
- owner 過濾：一律 `user_id`

## 8. 前端

- 搜尋框不變（同框，維持現有 UX）
- `MeetingList` 每筆多顯示 `matchSnippet`（命中的逐字稿片段），query 詞高亮
- `MeetingsPage`：search 有值時渲染 snippet；無值時維持現有列表
- 重新生成 TS client（openapi 更新後）

## 9. 錯誤處理與降級

| 情境 | 處理 |
|------|------|
| 索引時 embedding API 失敗 | 走現有 retry；不影響已生成文件；下次 sweeper 重試 |
| 查詢時 query embedding 失敗 | **降級為純 ILIKE**，記 log，搜尋仍可用 |
| transcript 為空 | 不建 chunks（跳過索引） |
| 會議刪除 | chunks 由 `ON DELETE CASCADE` 自動清 |

外部錯誤原文只進 log，不暴露給用戶端（資料安全規範）。

## 10. 測試策略（TDD）

- **chunker 純函數**：句界、~400 字邊界、overlap、空 / 短文本 → 單元測試
- **IndexUC**：mock Embedder + mock ChunkRepository → 冪等（已有 chunks 跳過）、逐塊 embed
- **SearchUC**：mock Embedder + mock ChunkRepository → 合併去重、降級（embed 失敗回純字面）
- **chunk_repo（pgvector）**：真實 PG 整合測試——本地 compose 換 `pgvector/pgvector:pg16` image；測 Upsert / SearchSimilar cosine 排序 / owner 過濾 / DeleteByMeeting
- **embedding 品質**：人工驗收（搜「定價」找到「價格策略」），不做自動測

## 11. 成本

- **儲存**（向量存 Neon）：不額外付費。768 維 ≈ 3KB/塊，每會議數十~百 KB，Neon free tier（0.5GB）綽綽有餘
- **查詢**（pgvector）：不額外付費，算在 Neon compute
- **Embedding API**（Gemini）：用現有 key，free tier 涵蓋；付費也是文件生成的零頭。每會議索引一次性、查詢一次 query embedding

## 12. 前置與風險

- **本地**：`docker-compose.yml` 的 PG image 換 `pgvector/pgvector:pg16`
- **Neon**：跑 `CREATE EXTENSION vector`（migration 內含 `CREATE EXTENSION IF NOT EXISTS`）
- **CI**：GitHub Actions 的 PG service image 換 pgvector 版
- **Go 依賴**：新增 `github.com/pgvector/pgvector-go`
- **維度鎖定**：一旦選 768 維，換模型 / 維度需重新 embed 全部（migration 重建）

## 13. 驗收條件

- [ ] 逐字稿處理完成後自動切塊 + embedding + 存 `transcript_chunks`
- [ ] retry / 重啟不重複索引（冪等）
- [ ] 既有已完成會議由回填掃描自動索引
- [ ] 搜尋框輸入自然語言，回傳語意相關會議（搜「定價」找到含「價格策略」的會議）
- [ ] 字面命中（人名 / 專有名詞）仍準確（hybrid 未退化字面能力）
- [ ] 每筆結果顯示命中的逐字稿片段 snippet
- [ ] 僅本人會議可被搜到（owner 過濾）
- [ ] 查詢時 embedding 服務失敗，降級為純 ILIKE 仍可搜
- [ ] 會議刪除後其 chunks 一併清除
```
