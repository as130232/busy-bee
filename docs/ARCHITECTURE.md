# Busy Bee 架構說明

> 最後更新：2026-07-17
>
> **注意**：本文件描述已核准的目標架構。標有**（計畫中，尚未實作）**的模組尚未完成；隨 `docs/PLAN.md` 各 Phase 完成後逐步移除標註並補實作細節。Phase 1（骨架 / apperr / config / response / compose）已實作。

---

## 目錄

1. [技術選型](#1-技術選型)
2. [整體架構概覽](#2-整體架構概覽)
3. [模組依賴關係](#3-模組依賴關係)
4. [核心資料模型 / 介面](#4-核心資料模型--介面)
5. [資料流 / 請求生命週期](#5-資料流--請求生命週期)
6. [外部依賴](#6-外部依賴)
7. [錯誤處理](#7-錯誤處理)
8. [後端分層說明](#8-後端分層說明)
9. [設計決策（ADR）](#9-設計決策adr)
10. [已取消 / 暫緩設計](#10-已取消--暫緩設計)
11. [計畫中但尚未實作](#11-計畫中但尚未實作)
12. [開發指令](#12-開發指令)

---

## 1. 技術選型

| 類別 | 選擇 | 主要原因 |
|------|------|---------|
| 後端語言 | Go 1.26 | 與 sport-hub 一致，開發者主力語言 |
| HTTP framework | Gin v1.12.x | 與 sport-hub 一致，middleware 生態完整 |
| 資料庫 | PostgreSQL（Cloud SQL） | 關聯資料 + 未來 pgvector 升級路徑 |
| DB 存取 | sqlc + pgx v5 | Type-safe SQL，無 ORM（ADR-005） |
| Migration | golang-migrate | CLI + embedded，CI 可跑 |
| 任務佇列 | Asynq + Redis | 持久化、retry、延遲任務三合一（ADR-003） |
| Redis 託管 | Upstash（MVP） | 成本考量（ADR-008） |
| 即時通訊 | WebSocket（nhooyr.io/websocket） | 預留即時逐字稿升級空間（ADR-002） |
| Logger | slog（stdlib） | 結構化，無額外依賴 |
| 設定管理 | env-based（.env.local） | 同 sport-hub，無 YAML |
| STT | Groq Whisper Large v3 | 成本低、速度快、中英夾雜佳 |
| LLM | Gemini 3.0-flash（`google.golang.org/genai`） | 已有 key；interface 隔離可抽換（ADR-007） |
| 音訊儲存 | GCS | 與 Cloud Run 同區低延遲 |
| 身份驗證 | Firebase Auth（Google Login） | 小團隊快速落地 |
| 前端 | React + Vite（PWA） | |
| 前端部署 | Firebase Hosting | |
| 後端部署 | GCP Cloud Run | min-instances=1（ADR-004） |

---

## 2. 整體架構概覽

Clean Architecture，依賴方向只能由外往內（對齊 sport-hub 的層次命名）：

```text
┌────────────────────────────────────────────┐
│  interface 層（HTTP handler / WS hub）      │ ← 接受請求、格式化回應
├────────────────────────────────────────────┤
│  application 層（Use Case）                 │ ← 業務流程協調、transaction boundary
├────────────────────────────────────────────┤
│  domain 層（Entity / Port Interface）       │ ← 核心業務規則，零外部依賴
├────────────────────────────────────────────┤
│  infrastructure 層（DB/GCS/STT/LLM/Queue）  │ ← 技術實作，注入到上層
└────────────────────────────────────────────┘

worker（Asynq handler）與 interface 層同位階，呼叫 application 層。
HTTP server 與 Asynq worker 跑在同一個 binary（ADR-004）。
```

---

## 3. 模組依賴關係

```text
interface/http/handler/*        worker/process_meeting
        ↓                              ↓
   application/meeting  ────────────────
        ↓
   domain/{meeting,user,artifact}（entity + port interfaces）
        ↑
   infrastructure/{db,gcs,stt,llm,queue}   ← 注入端（實作 domain interfaces）
```

**規則：**
- 任何模組禁止反向依賴：domain 不 import application / infrastructure / interface
- infrastructure 各子模組互不依賴
- `pkg/`（apperr / errcode / ctxutil）是橫向公用，可被任何層使用，但不得 import 業務模組

---

## 4. 核心資料模型 / 介面

### Meeting（domain/meeting/meeting.go）（計畫中，尚未實作）

```text
type Meeting struct {
    ID              uuid.UUID
    UserID          uuid.UUID
    Title           string
    AudioGCSPath    string
    Status          Status   // scheduled|pending|transcribing|analyzing|completed|failed
    Transcript      string
    DurationSeconds int
    ErrorMessage    string
    ScheduledAt     *time.Time
    RemindBeforeMin int      // 提醒提前分鐘數，預設 15（PRODUCT.md Q3）
    ProcessedAt     *time.Time
}
```

狀態機：`scheduled → pending → transcribing → analyzing → completed | failed`

### Port interfaces（計畫中，尚未實作）

```text
// domain/meeting/repository.go
type Repository interface {
    Create / GetByID / UpdateStatus / SaveTranscript / ListByUser / SearchByKeyword ...
}

// domain/meeting/stt.go
type STTClient interface {
    Transcribe(ctx, audioReader, mimeType) (transcript string, err error)
}

// domain/artifact/llm.go
type LLMClient interface {
    GeneratePRD(ctx, transcript) (string, error)
    GenerateTechSpec(ctx, transcript) (string, error)
}
```

### 資料表

`users`、`meetings`、`artifacts`（artifact_type：`prd` | `tech_spec`）、`push_subscriptions`。
欄位明細見 migration 檔（`busy-bee-be/db/migrations/`）；ER 圖見 `docs/superpowers/specs/2026-07-17-busy-bee-mvp-design.md §4`。

---

## 5. 資料流 / 請求生命週期

一場會議從上傳到產出文件的完整生命週期（計畫中，尚未實作）：

1. **建立**：FE `POST /meetings` → auth middleware 驗 JWT → application 建 meeting 記錄 → 回 GCS signed upload URL
2. **直傳**：FE `PUT {signed URL}` 直傳 GCS（不經後端，ADR-001）
3. **觸發**：FE `POST /meetings/{id}/complete-upload` → application 驗證物件存在 → enqueue Asynq 任務 → status = pending
4. **轉錄**：worker 下載音訊 →（超過上限則 ffmpeg 壓縮）→ Groq STT → 存 transcript → status: transcribing → analyzing
5. **生成**：Gemini 產出 PRD → 存 artifact → 產出 Tech Spec → 存 artifact → status = completed
6. **通知**：每次狀態變更 → Redis Pub/Sub publish → 各 instance WS hub 轉發給該 user 的連線
7. **失敗**：任一階段失敗 → Asynq retry（冪等，ADR-009）→ 達上限 → status = failed + error_message

對應 PRODUCT.md 功能：F-UPLOAD（1-3）、F-PIPELINE（4）、F-DOCGEN（5）、F-STATUS（6）

---

## 6. 外部依賴

| 依賴 | 角色 | failover 策略 |
|------|------|--------------|
| Cloud SQL（PostgreSQL） | 主資料庫 | 斷線時 API 回 503；無讀寫分離（單庫） |
| Upstash Redis | Asynq 佇列 + Pub/Sub | 斷線時無法 enqueue → complete-upload 回 503；WS 通知降級為前端重連拉取 |
| GCS | 音訊儲存 | 斷線時無法產 signed URL → 上傳功能回 503 |
| Groq API | STT | Asynq retry（exponential backoff）；達上限標 failed，可手動 retry |
| Gemini API | LLM 文件生成 | 同上；LLMClient interface 隔離，可換供應商（ADR-007） |
| Firebase Auth | JWT 驗證 | Admin SDK 本地驗簽（public key cache），Firebase 短暫不可用不影響已簽發 token 驗證 |
| Secret Manager | API keys | 啟動時載入；載入失敗直接 fail fast，拒絕啟動 |

---

## 7. 錯誤處理

apperr 模式（移植自 sport-hub `pkg/apperr/`）（計畫中，尚未實作）：

- `apperr.New(errcode.XXX, params...)` — 業務錯誤
- `apperr.Wrap(cause, errcode.XXX)` — 包裝外部錯誤（DB / API）；原始 cause 只進 log，不回傳用戶端
- handler 層統一由 `response.Fail` 轉換：errcode → HTTP status + 統一 envelope（`errCode` / `msg` / `data` / `traceId`）
- 錯誤碼定義於 `pkg/consts/errcode/`

---

## 8. 後端分層說明

| 層 | 位置 | 職責 | 禁止 |
|----|------|------|------|
| domain | `busy-bee-be/domain/` | Entity、狀態機、port interface | import 任何外部套件與其他層 |
| application | `busy-bee-be/application/` | Use case 協調、transaction boundary（WithTx） | 直接操作 DB / HTTP |
| infrastructure | `busy-bee-be/infrastructure/` | db / gcs / stt / llm / queue 實作 | 被 domain import |
| interface | `busy-bee-be/interface/http/` | handler、middleware、route、response、WS hub | 寫業務邏輯 |
| worker | `busy-bee-be/worker/` | Asynq handler，呼叫 application | 繞過 application 直接操作 infra |
| pkg | `busy-bee-be/pkg/` | apperr / errcode / ctxutil 橫向公用 | import 業務模組 |

Transaction boundary 一律在 application 層（`WithTx` pattern），repository 不開 transaction。

---

## 9. 設計決策（ADR）

#### ADR-001: 音訊上傳走 GCS Signed URL 直傳

- **狀態**：採納
- **背景**：Cloud Run HTTP/1 request 上限 32MB，一小時會議音訊約 30-60MB，multipart 經後端上傳必然失敗。
- **決策**：三段式流程——`POST /meetings` 取得 signed URL → FE 直傳 GCS → `POST /meetings/{id}/complete-upload` 觸發處理。Signed URL 綁 Content-Length ≤ 200MB 與 audio/* content-type 白名單；bucket 設 CORS 允許 FE domain。
- **後果**：繞過大小限制、後端零大檔案記憶體壓力；代價是前端多一段上傳邏輯、bucket 需 CORS 設定。
- **替代方案**：multipart 經後端 → 否決：撞 32MB 上限；Cloud Run HTTP/2 streaming → 否決：複雜度高且仍占用後端資源。

#### ADR-002: 即時通知用 WebSocket + Redis Pub/Sub fan-out

- **狀態**：採納
- **背景**：處理管線是非同步的，前端需要即時知道狀態；Cloud Run 多 instance 下，worker 與目標 WS 連線可能不在同一 instance。
- **決策**：WebSocket 作為前端通道（保留未來即時逐字稿能力）；worker 狀態變更 publish 到 Redis Pub/Sub，各 instance 的 WS hub 訂閱後轉發給自己持有的連線。瀏覽器 WS 不能帶 Authorization header，改為連線後第一則訊息帶 Firebase JWT，驗證通過前不綁定 user。前端 hook 自動重連並於重連後主動拉取最新狀態。
- **後果**：多 instance 正確送達、零額外基礎設施（Redis 已因 Asynq 存在）；代價是 hub 的 goroutine 生命週期管理需謹慎（人工必審）。
- **替代方案**：SSE → 否決：使用者選擇 WS 以預留雙向即時場景；輪詢 → 否決：延遲與 API 浪費。

#### ADR-003: 任務佇列用 Asynq（Redis-backed）

- **狀態**：採納
- **背景**：STT + LLM 處理耗時數分鐘，必須背景化；Cloud Run 縮容會中斷 in-process 任務。
- **決策**：Asynq——任務持久化、retry、延遲任務（ProcessAt，供 F-REMIND 用）一套解決。
- **後果**：任務不因重啟遺失；代價是多一個 Redis 依賴。
- **替代方案**：in-process worker pool → 否決：縮容遺失任務；GCP Cloud Tasks → 否決：本地開發模擬麻煩、無延遲任務取消重排的順手 API。

#### ADR-004: HTTP server 與 Asynq worker 同 binary、同 Cloud Run service

- **狀態**：採納（流量成長後重新評估拆分）
- **背景**：Cloud Run 是 request-driven；Asynq worker 主動從 Redis 拉任務，無 HTTP 流量時 instance 被凍結，任務不會被處理。
- **決策**：同 binary 同 service，設 `min-instances=1` + CPU always allocated。
- **後果**：部署簡單、成本可控（單常駐 instance）；代價是 HTTP 與 worker 資源共享，重負載互相影響。
- **替代方案**：獨立 worker service → 否決：MVP 階段成本與維運翻倍，無此需求。

#### ADR-005: sqlc + pgx，不用 ORM

- **狀態**：採納
- **背景**：與 sport-hub 一致的資料存取風格；展示 SQL 能力。
- **決策**：query 寫在 `db/query/*.sql`，sqlc 產生 type-safe Go 函數，pgx v5 連線。
- **後果**：SQL 可控、效能透明；代價是 schema 變更需重跑 codegen。
- **替代方案**：GORM → 否決：隱藏 SQL、面試展示價值低。

#### ADR-006: 全文搜尋 MVP 用 ILIKE

- **狀態**：採納（資料量成長或需要語意搜尋時升級）
- **背景**：PostgreSQL tsvector 預設不斷中文詞；pg_jieba extension 在 Cloud SQL 上有可用性風險。
- **決策**：MVP 用 `ILIKE '%keyword%'` 搜 title + transcript，數千筆內效能足夠。
- **後果**：零額外依賴、立即可用；代價是無排名、大資料量下變慢。
- **替代方案**：tsvector + pg_jieba → 否決：Cloud SQL extension 支援不確定；Meilisearch → 否決：多一個服務要管。**升級路徑**：pgvector（Cloud SQL 原生支援）+ Gemini embedding，新增 transcript_chunks 表做 RAG 語意搜尋，不動現有 schema。

#### ADR-007: LLM 用 Gemini 3.0-flash，以 domain interface 隔離

- **狀態**：採納
- **背景**：使用者已有 Gemini API key；長 transcript 需要大 context window。
- **決策**：`google.golang.org/genai` SDK 實作 `domain/artifact.LLMClient` interface；prompt template 與 client 分離。
- **後果**：換供應商只動 `infrastructure/llm/`；代價是 prompt 需針對 Gemini 調校。
- **替代方案**：Claude Sonnet → 否決（MVP 階段）：需另購 API，interface 已預留切換空間。

#### ADR-008: Redis 用 Upstash，不用 GCP Memorystore

- **狀態**：採納（有量後重新評估）
- **背景**：Memorystore 最低階 1GB 約 $35/月，是 side project 最大單筆固定開銷。
- **決策**：MVP 用 Upstash Redis（serverless、free tier、Asynq 相容）。全套月費目標 ≤ $25。
- **後果**：成本大幅下降；代價是跨網路延遲略高於同 VPC 的 Memorystore（佇列場景可接受）。
- **替代方案**：Memorystore → 否決：成本；自架 e2-micro → 否決：維運負擔。

#### ADR-009: worker 任務冪等性——分階段 status 檢查

- **狀態**：採納
- **背景**：Asynq retry 一個做到一半的任務會重複呼叫 Groq/Gemini（重複花錢）、產生重複 artifacts。
- **決策**：worker 每個階段開始前先查 meeting 當前 status 與產物是否存在（transcript 已存在則跳過 STT；artifact 已存在則跳過該生成），使任務天然冪等。
- **後果**：retry 零成本、失敗恢復乾淨；代價是每階段多一次 DB 查詢（可忽略）。
- **替代方案**：任務去重 key → 否決：擋不住「中途失敗後重跑前半段」的重複扣費。

---

## 10. 已取消 / 暫緩設計

| 項目 | 狀態 | 理由 | 對應 PLAN.md |
|------|------|------|-------------|
| OTel tracing / circuit breaker / singleflight / L1L2 快取 | ⏸ 暫緩 | sport-hub 的進階模式，本專案流量不需要，避免過度設計 | — |
| 即時逐字稿 streaming | ⏸ 暫緩 | WebSocket 架構已預留；等 MVP 驗證後再評估 | — |
| pgvector 語意搜尋 | ⏸ 暫緩 | post-MVP 升級路徑，見 ADR-006 | — |

---

## 11. 計畫中但尚未實作

> 目前**整個系統**均為計畫中。下表列主要模組與對應 Phase，完成後逐項移除。

| 項目 | 規劃位置 | 對應 PRODUCT.md | 預計 Phase |
|------|---------|----------------|----------|
| users 表 + auth middleware + /users/sync | `busy-bee-be/db/`, `busy-bee-be/interface/http/middleware/` | F-AUTH | Phase 2 |
| 前端登入 + Dashboard | `busy-bee-fe/src/` | F-AUTH | Phase 3 |
| CI/CD + Cloud Run + Hosting | `.github/workflows/`, `busy-bee-be/Dockerfile` | — | Phase 4 |
| GCS signed URL 上傳流程 | `busy-bee-be/infrastructure/gcs/` | F-UPLOAD | Phase 5 |
| Asynq + Groq STT 管線 | `busy-bee-be/worker/`, `busy-bee-be/infrastructure/stt/` | F-PIPELINE | Phase 6 |
| WS hub + Redis Pub/Sub | `busy-bee-be/interface/http/handler/ws.go` | F-STATUS | Phase 7 |
| useRecorder 錄音 | `busy-bee-fe/src/hooks/` | F-RECORD | Phase 8 |
| Gemini 文件生成 | `busy-bee-be/infrastructure/llm/` | F-DOCGEN | Phase 9 |
| 歷史 / 搜尋 | `busy-bee-be/application/meeting/search.go` | F-HISTORY、F-SEARCH | Phase 10 |
| Web Push 提醒 | `busy-bee-fe/public/sw.js` 等 | F-REMIND | Phase 11 |

---

## 12. 開發指令

> 以下指令隨 Phase 1-2 建立後生效（計畫中，尚未實作）。

```bash
# 本地環境
docker compose up -d              # PostgreSQL + Redis

# 後端（busy-bee-be/）
go run ./cmd/server               # 啟動 HTTP + worker
go build ./... && go vet ./...    # 編譯與靜態檢查
go test ./...                     # 測試
sqlc generate                     # db/query/*.sql 變更後重新產生
migrate -path db/migrations -database "$DB_URL" up   # 跑 migration

# 前端（busy-bee-fe/）
npm run dev / build / typecheck / lint
```

sqlc / migration 變更 SOP：改 `db/migrations/` 或 `db/query/*.sql` 後必須重跑 `sqlc generate` 並 commit 產出。
