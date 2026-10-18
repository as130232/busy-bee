# Busy Bee 架構說明

> 最後更新：2026-10-18
>
> **注意**：全部 Phase（1-12）均已實作並上線（production：https://busy-bee-502710.web.app）。「已取消 / 暫緩設計」與「post-MVP」項目見〈已取消 / 暫緩設計〉。

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
13. [設定與環境變數](#13-設定與環境變數)
14. [安全邊界](#14-安全邊界)
15. [測試策略](#15-測試策略)
16. [可觀測性](#16-可觀測性)
17. [部署與回滾](#17-部署與回滾)

---

## 1. 技術選型

| 類別 | 選擇 | 主要原因 |
|------|------|---------|
| 後端語言 | Go 1.26 | 與 sport-hub 一致，開發者主力語言 |
| HTTP framework | Gin v1.12.x | 與 sport-hub 一致，middleware 生態完整 |
| 資料庫 | PostgreSQL（本地 Docker；production：Neon 免費方案，Singapore） | 關聯資料 + pgvector 升級路徑；$0、直連（非 pooler，pgx prepared statement 相容） |
| DB 存取 | sqlc + pgx v5 | Type-safe SQL，無 ORM（ADR-005） |
| Migration | golang-migrate | CLI + embedded，CI 可跑 |
| 任務佇列 | in-process 記憶體佇列 + DB 復原掃描 | 無 Redis、$0、單 instance 足夠（ADR-010，取代 ADR-003） |
| 即時通訊 | WebSocket（nhooyr.io/websocket） | 預留即時逐字稿升級空間（ADR-002，單 instance 免 Pub/Sub） |
| Logger | slog（stdlib） | 結構化，無額外依賴 |
| 設定管理 | env-based（.env.local） | 同 sport-hub，無 YAML |
| STT | Deepgram nova-2（`language=zh-TW`、diarize=true） | 聲學語者分離（分講者逐字稿）；Groq/Gemini client 保留未 wire，換供應商只動 `infrastructure/stt/`（ADR-011） |
| LLM | Gemini（`google.golang.org/genai`；預設 gemini-3-flash-preview + thinkingLevel=LOW；404 時自動備援 `GEMINI_FALLBACK_MODEL`（預設 gemini-3.5-flash），ADR-017） | 已有 key；interface 隔離可抽換（ADR-007） |
| 音訊儲存 | GCS | 與 Cloud Run 同區低延遲 |
| 身份驗證 | Firebase Auth（Google Login） | 小團隊快速落地 |
| 前端 | React + Vite（PWA） | |
| 前端部署 | Firebase Hosting | |
| 後端部署 | GCP Cloud Run | min-instances=0 scale-to-zero（ADR-004 修訂）+ no-cpu-throttling |
| 提醒觸發 | Cloud Scheduler → `/internal/sweep-reminders` | scale-to-zero 下 instance 常為 0，進程內 ticker 不可靠；改由 Scheduler 每分鐘喚醒 instance 觸發掃描（密鑰保護，冪等）|

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

worker（Sweeper 與佇列 worker pool）與 interface 層同位階，呼叫 application 層。
HTTP server 與佇列 worker 跑在同一個 binary（ADR-004、ADR-010）。
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

### Meeting（domain/meeting/meeting.go）

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

### Port interfaces

```text
// domain/meeting/meeting.go — 主 port + 窄介面（ISP）
type Repository interface {
    Create / GetForUser / Get / UpdateStatus / SaveTranscript /
    SetCompleted / SetFailed / ListUnfinishedIDs / ListForUser
}
type ScheduleRepository interface { CreateScheduled / UpdateSchedule }
type ReminderRepository interface { ListDueReminders / MarkReminded }

// domain/meeting/stt.go
type STTClient interface {
    Transcribe(ctx, audio io.Reader, sizeBytes, filename) (TranscribeResult, error)
}

// domain/artifact/artifact.go
type LLMClient interface {
    GeneratePRD(ctx, transcript) (string, error)
    GenerateTechSpec(ctx, transcript) (string, error)
}

// domain/push/push.go
type Sender interface { Send(ctx, Subscription, Message) error }

// domain/meeting/notifier.go、queue.go、storage.go
StatusNotifier / TaskQueue / AudioStorage
```

### 資料表

`users`、`meetings`（含 `scenario` 情境、`summary` 一句話、`summary_sections` jsonb 結構化區塊、`transcript_segments`/`speaker_names`）、`artifacts`（artifact_type：`prd` | `tech_spec` | `action_items`；PRD/TechSpec 已改選用、預設不產）、`action_items`、`transcript_chunks`、`push_subscriptions`。
欄位明細見 migration 檔（`busy-bee-be/db/migrations/`）；ER 圖見 `docs/superpowers/specs/2026-07-17-busy-bee-mvp-design.md §4`。

---

## 5. 資料流 / 請求生命週期

一場會議從上傳到產出文件的完整生命週期：

1. **建立**：FE `POST /meetings` → auth middleware 驗 JWT → application 建 meeting 記錄 → 回 GCS signed upload URL
2. **直傳**：FE `PUT {signed URL}` 直傳 GCS（不經後端，ADR-001）
3. **觸發**：FE `POST /meetings/{id}/complete-upload` → application 驗證物件存在 → 排入記憶體佇列 → status = pending
4. **轉錄**：worker 下載音訊 → Deepgram STT（diarize）→ 依 speaker 聚合成 A/B/C 片段 → 攤平成帶講者前綴文字存 transcript + transcript_segments（空稿標 failed 可重試）→ status: transcribing → analyzing
5. **生成**：analyzing 階段依 `scenario` 選 prompt → Gemini 產出結構化摘要區塊（存 `summary_sections`）＋抽取一句話摘要與行動項（各自冪等）→ status = completed（PRD/Tech Spec 改選用、不自動產，ADR-012）
6. **通知**：每次狀態變更 → in-process notifier → 單機 WS hub 轉發給該 user 的連線（ADR-010 單 instance）
7. **失敗**：任一階段失敗 → 記憶體佇列 backoff retry（冪等，ADR-009）→ 達上限 → status = failed；重啟遺失由 Sweeper 掃 DB 復原

對應 PRODUCT.md 功能：F-UPLOAD（1-3）、F-PIPELINE（4）、F-DOCGEN（5）、F-STATUS（6）

---

## 6. 外部依賴

| 依賴 | 角色 | failover 策略 |
|------|------|--------------|
| Neon（PostgreSQL，production） | 主資料庫 | 斷線時 API 回 503；scale-to-zero 喚醒由 pgx pool 重連吸收 |
| GCS | 音訊儲存 | 斷線時無法產 signed URL → 上傳功能回 503 |
| Deepgram API | STT（語者分離） | 記憶體佇列 backoff retry；空稿/達上限標 failed，Sweeper 週期復原；換供應商只動 `infrastructure/stt/`（ADR-011） |
| Gemini API | LLM 文件生成 | 同上；LLMClient interface 隔離，可換供應商（ADR-007） |
| Firebase Auth | JWT 驗證 | Admin SDK 本地驗簽（public key cache），Firebase 短暫不可用不影響已簽發 token 驗證 |
| Secret Manager | API keys | 啟動時載入；載入失敗直接 fail fast，拒絕啟動 |

---

## 7. 錯誤處理

apperr 模式（移植自 sport-hub `pkg/apperr/`）：

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
| worker | `busy-bee-be/worker/` | Sweeper 復原掃描；佇列 worker pool 由 main 組裝 | 繞過 application 直接操作 infra |
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

- **狀態**：採納（**2026-07-18 修訂**：ADR-010 鎖定 max-instances=1 後不需跨 instance fan-out，Redis Pub/Sub 取消，改 in-process notifier 直通單機 WS hub；WS 通道、第一則訊息 JWT 驗證、前端重連補狀態等設計不變）
- **背景**：處理管線是非同步的，前端需要即時知道狀態；Cloud Run 多 instance 下，worker 與目標 WS 連線可能不在同一 instance。
- **決策**：WebSocket 作為前端通道（保留未來即時逐字稿能力）；worker 狀態變更 publish 到 Redis Pub/Sub，各 instance 的 WS hub 訂閱後轉發給自己持有的連線。瀏覽器 WS 不能帶 Authorization header，改為連線後第一則訊息帶 Firebase JWT，驗證通過前不綁定 user。前端 hook 自動重連並於重連後主動拉取最新狀態。
- **後果**：多 instance 正確送達、零額外基礎設施（Redis 已因 Asynq 存在）；代價是 hub 的 goroutine 生命週期管理需謹慎（人工必審）。
- **替代方案**：SSE → 否決：使用者選擇 WS 以預留雙向即時場景；輪詢 → 否決：延遲與 API 浪費。

#### ADR-003: 任務佇列用 Asynq（Redis-backed）

- **狀態**：**已取代（被 ADR-010 取代）**
- **背景**：STT + LLM 處理耗時數分鐘，必須背景化；Cloud Run 縮容會中斷 in-process 任務。
- **決策**：Asynq——任務持久化、retry、延遲任務（ProcessAt，供 F-REMIND 用）一套解決。
- **後果**：任務不因重啟遺失；代價是多一個 Redis 依賴。
- **替代方案**：in-process worker pool → 當時否決理由「縮容遺失任務」未考慮 DB 復原掃描，後由 ADR-010 推翻。

#### ADR-004: HTTP server 與 Asynq worker 同 binary、同 Cloud Run service

- **狀態**：採納（**2026-07-19 修訂**：改 `min-instances=0`（scale-to-zero）。原常駐理由是 Asynq 需輪詢 Redis；ADR-010 改記憶體佇列 + Sweeper 後，instance 回收造成的處理中斷可由下次喚醒的 Sweeper 復原（冪等），常駐已無必要。維持 `--no-cpu-throttling` 讓 instance 存活期間背景處理有 CPU。月費 ~$12 → ~$0-2；代價：無人在線時文件生成延後到下次喚醒）
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

#### ADR-013: 跨會議 RAG 問答（F-QA）單次無狀態、複用檢索管線

- **狀態**：採納（Phase 19）
- **背景**：F-SEARCH 已建立 pgvector + Gemini embedding 的語意檢索（Phase 15）；使用者希望以自然語言對全部會議提問並得到帶來源引用的答案。
- **決策**：**單次問答、無狀態**（不新增資料表、不存對話歷史，最省成本、貼合 scale-to-zero）。資料流：`Embedder.Embed(問題)` → `QARetriever.SearchSimilarForQA`（top-K=8、`qaMinScore=0.6`、**不做 DISTINCT ON** 允許同會議多片段、JOIN meetings 取標題、`user_id` owner 過濾）→ 濾門檻後 1-based 重編號 → `Answerer.Answer`（prompt 要求僅依來源作答、以 `[n]` 標註出處、禁杜撰）。
- **成本護欄**：無片段通過門檻時**直接回提示、不呼叫 LLM**；問題長度 rune 截斷 500。
- **重用/淨新增**：重用 `Embedder`/`GeminiClient.complete`/`transcript_chunks`/handler 框架；淨新增 `Answerer`/`QARetriever` port（ISP，與 `ChunkRepository` 分離）、`SearchSimilarForQA`、`QAUC`、`handler/qa`、`prompts/cross_meeting_qa.md`、前端 AskPage。
- **引用 UX**：回應 `sources[]` 帶 `{index, meetingId, title, snippet}`，前端把答案中的 `[n]` 渲染成連往該會議的連結。
- **替代方案**：多輪對話（chat）→ 暫緩：需存 conversation + 每次帶前文，token 成本與複雜度高；串流回應 → 暫緩：MVP 先非串流。

#### ADR-007: LLM 用 Gemini 3.0-flash，以 domain interface 隔離

- **狀態**：採納
- **背景**：使用者已有 Gemini API key；長 transcript 需要大 context window。
- **決策**：`google.golang.org/genai` SDK 實作 `domain/artifact.LLMClient` interface；prompt template 與 client 分離。
- **後果**：換供應商只動 `infrastructure/llm/`；代價是 prompt 需針對 Gemini 調校。
- **替代方案**：Claude Sonnet → 否決（MVP 階段）：需另購 API，interface 已預留切換空間。

#### ADR-008: Redis 用 Upstash，不用 GCP Memorystore

- **狀態**：**廢棄**（被 ADR-010 取代：系統已無 Redis 依賴，託管問題不存在）
- **背景**：Memorystore 最低階 1GB 約 $35/月，是 side project 最大單筆固定開銷。
- **決策**：MVP 用 Upstash Redis（serverless、free tier、Asynq 相容）。全套月費目標 ≤ $25。
- **後果**：成本大幅下降；代價是跨網路延遲略高於同 VPC 的 Memorystore（佇列場景可接受）。
- **替代方案**：Memorystore → 否決：成本；自架 e2-micro → 否決：維運負擔。

#### ADR-010: 移除 Redis——記憶體佇列 + DB 復原掃描

- **狀態**：採納（取代 ADR-003；廢棄 ADR-008；修訂 ADR-002）
- **背景**：Asynq 常駐輪詢使 Upstash 免費額度必爆、計量方案反而昂貴；Memorystore $35+/月超出預算；自架 VM 增加維運。重新檢視發現：業務狀態的真相源一直在 Postgres（meetings.status），Redis 只是排程板。
- **決策**：in-process 記憶體佇列（`infrastructure/queue/memory.go`：in-flight 去重、backoff retry、背壓、graceful drain）+ `worker.Sweeper` 於啟動時與每 2 分鐘掃 DB 將未完成會議重新入列。鎖定 production max-instances=1。分階段冪等（ADR-009）保證重啟後重跑無害。
- **後果**：本地與 production 徹底無 Redis（少一容器、少一部署決策、$0）；WS 免 Pub/Sub。代價：鎖定單 instance（小團隊足夠）；重啟時進行中任務會重跑一次（冪等使其僅損失時間）；F-REMIND 改為掃描式排程。
- **替代方案**：e2-micro VM 自架 Redis → 否決：多一台要管的機器；Cloud Tasks → 否決：本地模擬麻煩，且記憶體方案已滿足需求。TaskQueue 為 domain port，未來規模需要時可無痛換回。

#### ADR-009: worker 任務冪等性——分階段 status 檢查

- **狀態**：採納
- **背景**：Asynq retry 一個做到一半的任務會重複呼叫 Groq/Gemini（重複花錢）、產生重複 artifacts。
- **決策**：worker 每個階段開始前先查 meeting 當前 status 與產物是否存在（transcript 已存在則跳過 STT；artifact 已存在則跳過該生成），使任務天然冪等。
- **後果**：retry 零成本、失敗恢復乾淨；代價是每階段多一次 DB 查詢（可忽略）。
- **替代方案**：任務去重 key → 否決：擋不住「中途失敗後重跑前半段」的重複扣費。

#### ADR-011: 語者分離（diarization）用 Deepgram nova-2，不用 Gemini 音訊

- **狀態**：採納（Phase 16）
- **背景**：需求為多人會議逐字稿分講者（A/B/C）。Groq Whisper 不支援 diarization，須換供應商。
- **決策**：用 **Deepgram nova-2 + `language=zh-TW` + diarize=true**（聲學語者分離，依聲紋分群），藏在既有 `STTClient` port 後。
- **後果**：一個聲音＝一位講者，單人不會被誤拆；換供應商只動 `infrastructure/stt/`。成本隨用隨付、零常駐，符合 scale-to-zero。
- **踩坑（保留紀錄）**：
  1. **Gemini 音訊做 diarization 不可靠**——它是 LLM「推測」講者（依內容/語氣），會把一個人硬拆成 A/B/C；架構性問題，prompt 調不好。已放棄。
  2. **Deepgram nova-3 + `language=multi` 對中文回傳空結果**（有 duration 但 words 空 → 逐字稿空白）。必須用 nova-2 + zh-TW。
- **替代方案**：AssemblyAI / ElevenLabs Scribe（同為聲學分離，未實測）；自架 pyannote → 否決（需 GPU 常駐，違反 scale-to-zero）。性別/年齡辨識不做（不可靠且無必要，代號＋手動改名已滿足）。

#### ADR-012: 紀錄情境化用「模板 + 結構化區塊」（B-lite），不做每情境專屬 UI

- **狀態**：採納（Phase 17，F-SCENARIO）
- **背景**：要把單一「會議」擴充為多情境（會議/閒聊，未來面試等）。若每情境刻一套專屬畫面，維護成本隨情境線性上升、產品分散。
- **決策（B-lite）**：擷取管線（STT + 分講者）情境無關；情境是 `meetings.scenario` 一個屬性，只決定：(1) 選哪個 LLM prompt 模板（`infrastructure/llm/prompts/summary_{scenario}.md`）、(2) 產出哪些「有型別的結構化區塊」（`SummarySection{type,title,items}`，存 `meetings.summary_sections` jsonb）。前端寫**一套通用區塊渲染器**（`SummarySections.tsx`）依 type 顯示。
- **情境化擴充點**：新增情境 = 新增一個 prompt 模板 + 在 `scenarioPrompts` map 註冊 + `Scenario` 常數，**不動前端、不動管線、不動 DB schema**。對齊 AGENTS.md「調 prompt 不動 client 邏輯」。
- **後果**：`Summarizer` port 與行動項 `Extractor` 分離、各一次 LLM 呼叫；PRD/Tech Spec 改為選用、不再自動產生（每筆 LLM 呼叫 3→2 次，降成本，符合 scale-to-zero 省費原則）。`generateArtifacts` 保留供日後 on-demand 觸發。
- **資料流變更**：analyzing 階段 = `generateSummarySections`（依情境 prompt，冪等：已有 sections 則跳過）+ `extractActionItems`（不變）。無效/空 scenario 一律 `ParseScenario` 回退 meeting（容忍舊資料）。
- **替代方案**：(A) 只換總結模板、畫面全同 → 差異化不足；(B-full) 每情境專屬 UI → 過重、分散。取中間 B-lite。

---

#### ADR-014: per-meeting STT 語言選擇，detect_language 而非 multi

- **狀態**：採納（Phase 25，F-LANGUAGE）
- **背景**：STT 語言原本是全域 config（`DEEPGRAM_LANGUAGE=zh-TW`）寫死，全英文會議/面試轉錄品質差（甚至可能拿到空逐字稿）。需求：每筆會議可選中文/英文/自動偵測。
- **決策**：language 變成 `meetings.language` 欄位（比照 ADR-012 scenario 的「屬性驅動」模式），`domain/meeting.STTClient.Transcribe` 加 `language` 參數；auto 選項用 Deepgram **`detect_language=true`**，偵測到中文時後製簡轉繁（`github.com/longbridgeapp/opencc`，go:embed 內嵌詞典，CPU 執行、無常駐服務，符合 scale-to-zero）。
- **踩坑（實測結論，保留紀錄）**：
  1. **`language=multi`（多語混講模式）對純中文音訊仍會壞掉**——用真實 Deepgram API + TTS 中文測試音檔實測，幾乎丟失全部內容（僅轉出兩個英文字）。ADR-011 當年記錄的雷至今沒修好，故完全不採用。
  2. **`detect_language=true` 是另一個獨立參數，正確可用**：中文音檔正確判斷 `zh`、英文音檔正確判斷 `en`；但 (a) 中文結果輸出為簡體，需後製轉繁；(b) 中英夾雜時英文單字斷詞比明確指定語言時差（如 "spr ing" 而非 "spring"）。
  3. **明確指定 `language=zh-TW`/`en-US` 轉錄品質最好**，維持為主選項；`auto` 僅作為使用者不確定語言時的降級選項。同時發現：明確指定語言但音訊實際是另一語言時，nova-3 會回**空逐字稿**（比 nova-2 亂轉還不易察覺），提醒語言選擇仍需使用者自行判斷。
- **後果**：`STTClient` port 簽名變更，三個實作（Deepgram/Groq/Gemini）皆需同步（後兩者未 wire，忽略參數即可編譯）；新增一個第三方 Go dependency（`longbridgeapp/opencc`，Apache-2.0，go:embed 內嵌詞典無外部檔案風險）。
- **替代方案**：只保留 zh-TW 硬編碼、不做 per-meeting 選擇 → 否決（英文使用者體驗差）；用 `multi` 模式一次涵蓋雙語 → 否決（實測會壞）；自建語音語言辨識模型 → 否決（過度設計，Deepgram 內建 detect_language 已夠用）。

#### ADR-015: STT 低信心字詞標記 + prompt 容錯，不讓 LLM 直接聽音檔

- **狀態**：採納（Phase 26，F-DOCGEN 品質強化）
- **背景**：報告只依 STT 逐字稿生成。收音差時 Deepgram 會出同音錯字/漏字，但 LLM 拿到的文字沒有任何可靠度資訊，錯字會被當事實寫進摘要，數字/人名尤其危險；漏掉的內容 LLM 完全看不到。
- **決策**：
  1. Deepgram 每個 word 的 `confidence` < 0.6（`lowConfidenceThreshold`）時，於 `TranscriptSegment.MarkedText` 以「〔?…〕」包住（相鄰低信心字詞合併成一段）。`Text` 保持乾淨供前端顯示與搜尋；`MarkedText` 只在段落含低信心字詞時才有值，存於 `transcript_segments` JSONB（omitempty，無 migration），**不對前端輸出**。
  2. 摘要（`timestampedTranscript`）與行動項（`analysisTranscript`）都改吃 `AnalysisText()`（有標記用標記版）。
  3. 所有摘要/行動項 prompt 說明逐字稿是 ASR 產物：可依上下文修正明顯同音錯字、不得補事實；依據不清的重點加註「（語音不清，待確認）」，數字/金額/日期/人名落在標記內不猜精確值。待確認以文字註記呈現，不改 JSON schema 與前端（ADR-012 B-lite）。
  4. 使用者手動編輯段落時清空該段 `MarkedText`（人工校正後標記失效）。
  5. nova-3 不支援 `keywords`，`buildDeepgramQuery` 對 `nova-3*` 改送 `keyterm`（去掉 `:權重` 後綴）；其他 model 維持 `keywords`。
  6. 中文模式（`zh*` 或 auto 偵測為中文）下，nova-3 會把夾雜的英文單字切成時間相連的片段（實測 Kubernetes → `k`/`er`/`net`/`es`，間隔 0 秒）；聚合時間隔 < 10ms（`fragmentGapSec`）的相鄰英文片段直接接合不加空白。英文模式不套用（英文單字本來就時間相連，接合會黏成一團）。拼字錯誤（如 kernetes）交由 LLM 依上下文修正。
- **成本**：同 Phase 對 `gemini-3*` 設 `thinkingLevel=LOW`（預設 high 的 thinking token 以輸出價計費；實測摘要每次省 1.1k–2.2k token，輸出品質無明顯差異）。
- **後果**：零額外 API 成本；只能標記「聽到但不確定」的字，STT 完全沒產出的內容仍會遺漏。門檻 0.6 為初值，需用真實低品質錄音觀察再調。
- **替代方案**：音檔＋逐字稿一起送 Gemini 多模態摘要 → 暫不採用（token 成本數倍、延遲增加、需改 `Summarizer` port）；若標記後仍常遺漏，可做成「平均 confidence 低於門檻才走多模態」的條件式降級。

#### ADR-016: AI 錄音重新校正——使用者觸發、伺服器分段、只改文字

- **狀態**：採納（Phase 27，F-CORRECT）
- **背景**：ADR-015 的低信心標記只能標「聽到但不確定」的字，STT 完全沒轉出的內容 LLM 看不到。讓 Gemini 每場都聽音檔成本數倍，改為使用者需要時才觸發。
- **決策**：
  1. 新表 `transcript_corrections`：每次一筆、`attempt` 1–2（每場上限 2，按下即計次）、部分唯一索引確保同場只有一筆進行中；存校正前快照（segments/transcript/summary_sections/summary）供整份還原，`progress` 存每段 patch 作斷點。
  2. 獨立記憶體佇列（併發 1）+ `NewSweeperFunc` 復原；`ExecuteUC` 冪等：終態跳過、progress 已有段不再呼叫 Gemini。
  3. 切段 `PlanChunks`：累積區間 ≤ 12 分鐘、對齊 segment 邊界；非首段音訊往前重疊 30 秒；上下文帶上一段最後 3 個 segment（已套用 patch，唯讀）。ffmpeg 只剪該段音訊上傳 Files API，費用與錄音長度成正比。
  4. Gemini 只回 `[{i,text}]` 有修改的段；`ValidatePatches` 丟棄越界、鎖定（`manuallyEdited`）、空字串、相同、長度比 ∉ [0.5, 2.0]；輸出無法解析（`ErrBadOutput`）視為無修改。講者代號與時間錨點不動（ADR-011）。
  5. 套用前重讀 meeting：校正期間被人工編輯的段跳過；`originalText` 只在首次被改時寫入；清 `MarkedText`。
  6. 摘要以整份校正後逐字稿重產；**tags 不動**（與手動標籤共用欄位）。行動項：`manual`/`done`/`edited` 保留，其餘 llm 項刪除換新抽取（`ExtractExcluding` 告知既有項 + 正規化去重）。`action_items.edited` 舊資料以 `updated_at > created_at` 保守回填。
  7. 跨表原子寫入沿用 `ProcessRepo` 模式（repo 內 `pgx.BeginFunc`）。狀態以 `StatusEvent.CorrectionStatus` 經 WS 推送，前端收到即重抓。
- **後果**：一小時錄音約 US$0.05–0.07、約 2 分鐘；新版前手動修改過的段落無 `manuallyEdited` 標記，可能被校正（標籤 + 還原緩解）。
- **替代方案**：每場自動多模態 → 否決（成本）；錄音中邊錄邊處理 → 否決（跨段講者代號無法對齊、iOS 背景上傳不穩）；同步 API 處理 → 否決（逾時、無斷點）；欄位全加在 meetings → 否決（只能留一份快照）。

#### ADR-017: 摘要保留可執行細節；模型固定 gemini-3-flash-preview + 自動備援

- **狀態**：採納（Phase 28）
- **背景**：Phase 26 依短逐字稿（4 行）A/B 改用 gemini-3.5-flash-lite，上線後使用者回報長會議報告「只知道大概討論什麼、細節全沒了」（例：「16:00 snapshot、隔日 11:00 發佈」被概括成「QA 測試與每日車次安排」）。
- **實測**（同一場 40 分鐘會議、校正後逐字稿、各跑兩次）：
  - 舊 prompt：3.5-flash-lite 決議 0–2 項、505–818 字（thinking high 亦同）；3-flash-preview 決議 3 項、870–1000 字；thinking low 與 high 細節相當 → **主因是 Lite 模型，不是 thinking**。
  - 新 prompt + 3-flash-preview（low）：5 區塊、約 1500–1670 字，保留時間/步驟/人員/工具；每次約 US$0.008–0.009（舊約 0.006）。
  - gemini-3.5-flash（GA）細節相當但成本 3–5 倍、兩次結果差異大；3.1-flash-lite 不遵守待確認規則 → 皆不採為主模型。
- **決策**：
  1. 主模型固定 `gemini-3-flash-preview`（不用浮動別名 gemini-flash-latest），`gemini-3*` 維持 thinkingLevel=LOW。
  2. Preview 可能下架：`GeminiClient.generateContent` 遇 404/NOT_FOUND 自動改用 `GEMINI_FALLBACK_MODEL`（預設 GA `gemini-3.5-flash`）並記 `llm.model_fallback` warn；其他錯誤照常交由佇列重試。摘要、行動項、AI 校正共用此路徑。
  3. 會議 prompt 目標由「掌握全貌」改為「讓沒參加的人能照著做事」：新增 `details`（流程與規範細節）、`open_questions`（待確認事項）區塊；具體性規則（必留時間/數字/人名/工具/步驟/條件，禁止空泛句）；`text` 上限 50 → 約 150 字、依會議長度增加條目；禁止自創 `[?…]` 等不確定標記。閒聊/面試 prompt 套用具體性規則與放寬字數（未新增區塊，尚未以真實錄音驗證）。
  4. 前端為通用渲染器（ADR-012 B-lite），新區塊不需改前端。
- **後果**：既有紀錄需重新處理或 AI 校正才會套用新 prompt；摘要約長 2.5 倍。
- **教訓**：模型/prompt 的 A/B 必須用真實長度（≥ 30 分鐘）的逐字稿，短樣本看不出概括與漏項。

---

#### ADR-018: 即時會議由瀏覽器直連 Deepgram 串流，後端無狀態

- **狀態**：採納（Phase 30，F-LIVE）
- **背景**：要在 Google Meet 網頁版會議中即時轉文字、整理重點與提問；使用者要求維持 scale-to-zero 省成本。Cloud Run 為 instance 計費（`--no-cpu-throttling`）、單一連線最長 60 分鐘。
- **決策**：
  1. 瀏覽器以後端簽發的 Deepgram 短效 JWT（`POST /v1/auth/grant`，TTL 30 秒，`Sec-WebSocket-Protocol: bearer, <jwt>`）直連 `wss://api.deepgram.com/v1/listen`；主金鑰只在後端（需 Member 以上權限）。
  2. 串流音訊用 16kHz 16-bit PCM（linear16，AudioWorklet 擷取），不用 webm：無容器標頭，斷線換新金鑰重連後可直接補送暫存（上限 30 秒）。
  3. 後端無狀態：即時逐字稿與目前重點存在瀏覽器，`/live/notes`、`/live/ask` 每次帶入；後端只負責 Gemini、月用量（心跳累加 `live_seconds`）、狀態轉換。會議中後端只在呼叫 Gemini 時喚醒。
  4. 狀態機：`（新建）/scheduled → live → scheduled（結束、待上傳音訊）→ pending → …`；結束後沿用既有 complete-upload 與處理管線，即時逐字稿不保存。
  5. 會前資料（`meeting_contexts`）轉純文字後放 prompt 最前面（隱性快取），並抽術語合併進 Deepgram keyterm；會後摘要、行動項、AI 校正也帶入，附「只用來理解、內容只能來自逐字稿」規則；無資料時 prompt 不變。
  6. 錄音另以混音 MediaRecorder（32 kbps）每 10 秒寫入 IndexedDB，分頁意外關閉可救回上傳。
  7. 音訊來源抽象 `AudioSource`（v1 `MeetTabSource`），之後 Teams 桌面版新增來源即可。
- **替代方案**：後端 WS 代轉 Deepgram（整場會議 instance 醒著、60 分鐘斷線，否決）；Gemini Live API 直接聽音訊（中文逐字稿與講者標示較差、按音訊 token 計費較貴、難控制更新頻率，否決）；`/live/ask` 用 SSE 串流（Firebase Hosting 轉 Cloud Run 是否緩衝未證實，v1 改一般請求，答案約 2–5 秒）。
- **實測（2026-10-15）**：nova-3 `language=zh-TW` 串流可用且輸出繁體；nova-2 zh-TW 串流連線失敗 → 串流模型獨立設定 `DEEPGRAM_LIVE_MODEL`（預設 nova-3）。Gemini 即時重點約 1.8 秒、提問約 4.6 秒。

---

## 10. 已取消 / 暫緩設計

| 項目 | 狀態 | 理由 | 對應 PLAN.md |
|------|------|------|-------------|
| Production 部署（Cloud Run / Hosting / CI-CD） | ⏸ 暫緩 | Supabase 免費額度滿，production DB 待定（Neon 或 Supabase 空位）；本機開發不受影響 | Phase 4.2-4.4 ⏸ |
| OTel tracing / circuit breaker / singleflight / L1L2 快取 | ⏸ 暫緩 | sport-hub 的進階模式，本專案流量不需要，避免過度設計 | — |
| 即時逐字稿 streaming | ✅ 完成 | Phase 30 F-LIVE：瀏覽器直連 Deepgram 串流（ADR-018）；即時逐字稿不保存，正式版仍走會後管線 | Phase 30 |
| pgvector 語意搜尋 | ⏸ 暫緩 | post-MVP 升級路徑，見 ADR-006 | — |
| 說話者分離（diarization，A/B/C 標記） | ✅ 完成 | Phase 16 以 Deepgram nova-2 聲學分離實作（ADR-011）；含講者改名、音檔播放。性別辨識不做（不可靠且無必要，代號＋改名已滿足） | Phase 16 |

---

## 11. 計畫中但尚未實作

> 全部 Phase 已實作完畢，本表清空。post-MVP 候選（pgvector 語意搜尋、diarization 等）記錄於 §10 暫緩表。

| 項目 | 規劃位置 | 對應 PRODUCT.md | 預計 Phase |
|------|---------|----------------|----------|

---

## 12. 開發指令

（本節為指令的唯一來源；AGENTS.md「常用指令」只列最常用幾條。2026-10-18 依 CI workflow 與程式現況校正：Redis 已於 ADR-010 移除、migration 改用內建 `cmd/migrate`、sqlc 設定在 `db/`）

```bash
# 本地環境
docker compose up -d              # PostgreSQL

# 後端（busy-bee-be/）
go run ./cmd/server               # 啟動 HTTP + worker
go build ./... && go vet ./...    # 編譯與靜態檢查
go test ./...                     # 測試
cd db && sqlc generate            # db/query/*.sql 變更後重新產生（設定檔 db/sqlc.yaml）
go run ./cmd/migrate up           # 跑 migration（讀 DB_URL；`down` 回退一版、`version` 看目前版本）
go test -tags integration ./infrastructure/db/... ./infrastructure/stt/...   # 整合測試（需 DB、ffmpeg）

# 前端（busy-bee-fe/）
npm run dev / build / typecheck / lint
npm run test:run                  # Vitest 單元測試
npm run gen:api                   # 依 busy-bee-be/api/openapi.yaml 重新產生 TS 型別
```

sqlc / migration 變更 SOP：改 `db/migrations/` 或 `db/query/*.sql` 後必須重跑 `sqlc generate` 並 commit 產出。

---

## 13. 設定與環境變數

- 載入：`infrastructure/config/config.go`；優先序 OS 環境變數 > `.env.{APP_ENV}` > 預設值。完整清單見根目錄 `.env.example`。
- 機敏值（`DB_URL`、`GEMINI_API_KEY`、`GROQ_API_KEY`、`DEEPGRAM_API_KEY`、`VAPID_PRIVATE_KEY`、`REMINDER_SWEEP_SECRET`）在 production 一律走 Secret Manager 掛到 Cloud Run，不寫進 repo 或 log。
- Deepgram 主金鑰須為 Member 以上權限（即時會議簽發短效金鑰需要，ADR-018）；production secret `busy-bee-deepgram-key`，Cloud Run 以固定版本引用（2026-10-15 起為 `:2`）。
- 即時會議參數：`DEEPGRAM_LIVE_MODEL`（預設 nova-3）、`LIVE_MONTHLY_MINUTES_LIMIT`（3000）、`LIVE_CONTEXT_MAX_CHARS`（100000）、`LIVE_CONTEXT_MAX_FILE_BYTES`（10MB）。
- 前端：`VITE_WS_BASE`（WebSocket 直連 Cloud Run 的網址，見 `busy-bee-fe/.env.production`）；Firebase Web 設定為公開值。

## 14. 安全邊界

- 認證：`/api/v1` 一律經 `middleware.Auth`（Firebase JWT 驗簽 + `ALLOWED_EMAILS` 白名單）與 `ResolveUser`；WS 第一則訊息驗 JWT（ADR-002）。
- 授權：業務查詢一律帶 `user_id`（見 AGENTS.md〈資料安全規範〉）；他人資源回 404，不區分不存在與無權限。
- 認證補充：白名單只認 token `email_verified=true` 的 email（HTTP 與 WS 皆同）。
- 限流：每位使用者（Firebase UID）10 req/s、burst 30，掛在 Auth 之後（`middleware.RateLimit`）；不以 IP 計，因 Gin 預設信任所有 proxy、`X-Forwarded-For` 可偽造。即時會議另有每場冷卻（重點 15 秒、提問 5 秒）與每月分鐘上限（ADR-018）。
- 請求大小：全域 body 上限 1MB（`middleware.BodyLimit`；會前資料上傳依檔案上限），超過回 413／41302；`http.Server` 設 `ReadHeaderTimeout` 10 秒、`IdleTimeout` 120 秒，不設讀寫逾時（WS 長連線）。
- 對外連線（SSRF）：依使用者網址發請求一律經 `infrastructure/netguard`（連線時檢查實際 IP，擋 loopback／私有網段／link-local 含 metadata／CGNAT）；yt-dlp 自行連網，只能事前 `CheckHost`（DNS rebinding 與 yt-dlp 自身轉址為已知殘餘風險）。推播端點限 FCM／Mozilla／Apple／WNS。
- WebSocket 來源：`FRONTEND_ORIGINS` 空時不檢查來源；prod 漏設會在啟動 log `config.insecure`。
- 回應：`middleware.SecurityHeaders`（prod 模式）；外部錯誤 cause 只進 log（`response.Fail`）。
- 上傳：音檔走 GCS signed URL，限制 content-type 與 200MB（ADR-001）；會前資料單檔 10MB、格式白名單。貼連結匯入與 AI 校正壓縮共用一個大檔暫存名額（`infrastructure/tempslot`），避免多個 200MB 檔同時落地 /tmp（Cloud Run 上算進 512Mi 記憶體）。
- 瀏覽器直連 Deepgram 只拿 30 秒短效 JWT，主金鑰不出後端（ADR-018）。
- 待確認：Firebase Hosting 尚未設定安全標頭與 CSP、Firebase API key 網域限制（PLAN 29.12）。

## 15. 測試策略

| 層 | 工具 | 範圍 |
|---|---|---|
| 後端單元 | `go test ./...` | domain / application 用 mock port；handler 用 httptest；LLM prompt 組裝與解析用假的 generate 函式 |
| 後端整合 | `go test -tags integration`（build tag，見 AGENTS.md〈測試規範〉） | repository 對真實 PostgreSQL、ffmpeg 壓縮切段；CI 對 db 與 stt 套件執行 |
| 前端單元 | Vitest + Testing Library（jsdom） | hooks、services、元件、`src/live/` 即時會議核心邏輯（假 WebSocket / MediaStream、fake-indexeddb） |
| 人工驗收 | 真機 / 真實會議 | 錄音、推播、即時會議等瀏覽器 API 與外部服務流程（各 Phase 的 e2e 項） |

CI（`.github/workflows/deploy.yml`）在部署前跑後端 vet + 單元 + 整合測試、前端 typecheck + lint + build；任一失敗不部署。

## 16. 可觀測性

- 結構化 log：`slog` JSON 輸出到 stdout（Cloud Logging 收集），等級由 `LOG_LEVEL` 控制。
- 每個請求由 `middleware.RequestID` 產生 traceId，`RequestLogger` 記錄；錯誤回應帶 `traceId` 方便對照 log。
- 處理管線、校正、即時會議只記 `meeting_id` 等識別欄位與數量、耗時；不記 token、逐字稿全文、會前資料內容。
- `GET /version` 回傳線上 commit 與建置時間，部署後用來確認版本。
- 待確認：前端錯誤監控（PLAN 29.12）、告警設定。

## 17. 部署與回滾

- 觸發：push 到 `main` → GitHub Actions：測試 → 建 image（tag = commit 前 7 碼）→ production migration（`go run ./cmd/migrate up`，DB_URL 取自 Secret Manager）→ `gcloud run deploy`（沿用服務既有的 env 與 secret 設定）→ 建前端並部署 Firebase Hosting。
- Cloud Run：scale-to-zero、`--no-cpu-throttling`、單 instance（ADR-004、ADR-010）。
- 回滾與維運操作步驟見 `docs/RUNBOOK.md`。

