# AI 錄音重新校正（F-CORRECT）設計

- 日期：2026-10-10
- 狀態：設計確認，待實作（Phase 27）
- 前置：Phase 26（低信心標記 `MarkedText`、prompt 待確認規則、ADR-015）已部署

## 1. 目標與成功標準

**目標**：收音差的紀錄，使用者可主動按「AI 校正」，讓 Gemini 直接聽已上傳的錄音修正逐字稿，再重產更完整的報告。成本只花在需要的紀錄。

**成功標準**
- 收音差的紀錄校正後，逐字稿錯字明顯減少、摘要不再漏掉重點。
- 使用者打勾、修改、手動新增的任何內容一律不被覆蓋或刪除。
- 單次校正成本可預測：一小時錄音約 US$0.05–0.07（gemini-3.5-flash-lite，音訊 32 token/秒、輸入 US$0.30/M、輸出 US$2.50/M）。

**不做（YAGNI）**
- 錄音中邊錄邊處理（跨段講者代號無法對齊，見對話結論）。
- 單段還原（只做整份還原）。
- Gemini 重做語者分離或時間錨點（ADR-011：LLM 推測式 diarization 會誤拆講者）。

## 2. 已確認的產品決策

| # | 決策 |
|---|------|
| D1 | 同一場紀錄最多校正 **2 次**；按下即計次，與是否還原無關 |
| D2 | 還原：逐字稿、摘要區塊、一句話摘要回到校正前；**行動項不動** |
| D2a | **校正不動標籤**：AI 標籤與手動標籤共用 `tags` 欄位、無法區分，重產會覆蓋使用者手動標籤；且標籤屬主題層級，校正錯字幾乎不影響（2026-10-10 撰寫計畫時修正） |
| D3 | 被改段落顯示「AI 已校正」標籤，點開看校正前原文；整份還原為單一按鈕 |
| D4 | 已完成紀錄一律顯示校正按鈕；低信心比例 ≥ 5% 時加建議提示 |
| D4a | 按下前的確認文案需說明「每場最多 2 次，失敗也會計次」 |
| D5 | 使用者手動改過的逐字稿段落：Gemini 可讀（上下文）、不可改；其餘段落可直接覆蓋 |
| D6 | 行動項：手動新增 / 已勾選 / 改過內容 → 保留；其餘 AI 抽取項 → 以新抽取結果覆蓋；新項目不得與保留項重複 |
| D7 | 伺服器端每約 12 分鐘切段、段間重疊 30 秒並帶上一段結尾作上下文、已處理段跳過、最後以整份逐字稿重產一次摘要 |
| D8 | 新版前手動改過的逐字稿段落無法辨識（未曾記錄），可能被校正；以「AI 已校正」標籤 + 整份還原 + 校正前提醒文案緩解 |

## 3. 資料結構

### 3.1 新表 `transcript_corrections`（migration 000017）

| 欄位 | 型別 | 說明 |
|------|------|------|
| `id` | uuid PK | |
| `meeting_id` | uuid FK → meetings ON DELETE CASCADE | |
| `user_id` | uuid FK → users ON DELETE CASCADE | 所有查詢以 user_id 過濾 |
| `attempt` | int | 1 或 2；`UNIQUE (meeting_id, attempt)`、`CHECK (attempt BETWEEN 1 AND 2)` |
| `status` | text | `queued` / `running` / `completed` / `failed` / `reverted`（CHECK） |
| `progress` | jsonb | `{"chunks": {"0": [{"i":12,"text":"…"}], "1": []}}`：已完成段索引 → 該段 patch |
| `original_segments` | jsonb | 校正前 `transcript_segments` 快照 |
| `original_transcript` | text | 校正前攤平逐字稿 |
| `original_summary_sections` | jsonb | 校正前摘要區塊 |
| `original_summary` | text | 校正前一句話摘要 |
| `error_message` | text | 僅存可對使用者顯示的訊息；原始 cause 只進 log |
| `created_at` / `completed_at` / `reverted_at` | timestamptz | |

- 部分唯一索引：`UNIQUE (meeting_id) WHERE status IN ('queued','running')` → 同場同時只有一筆進行中。
- 還原只允許「該場最新一筆且 `status='completed'`」。

### 3.2 `TranscriptSegment` JSON 新欄位（無 schema 變更）

- `manuallyEdited bool`（`json:"manuallyEdited,omitempty"`）：`EditSegmentUC` 設為 true。
- `originalText string`（`json:"originalText,omitempty"`）：被校正修改時存校正前文字；前端據此顯示「AI 已校正」。
- 被校正修改的段落清空 `MarkedText`。
- 還原時整份換回 `original_segments`，上述欄位自然回復。

### 3.3 `action_items` 新欄位（同 migration 000017）

- `edited boolean NOT NULL DEFAULT false`：`UpdateActionItemDescription` 時設 true。
- 舊資料回填：`UPDATE action_items SET edited = true WHERE updated_at > created_at`（保守：提醒寄送、勾選後取消也算改過 → 多保留不誤刪）。

### 3.4 API 回傳

- 校正狀態由獨立端點 `GET /api/v1/meetings/{id}/correction` 取得（POST 建立、revert 也回傳同結構）：`{ status, attemptsUsed, attemptsMax: 2, canRevert, errorMessage }`，無紀錄時 `attemptsUsed=0`、`status` 為空字串。不擴充 MeetingDetail，避免既有 GetUC 依賴 correction repo。
- MeetingDetail 加 `uncertainRatio`：number，0–1；讀取時由 segments 計算（`〔?…〕` 內字元數 ÷ 總字元數），不落庫。
- `transcriptSegments[]` 加 `manuallyEdited`、`originalText`（`MarkedText` 仍不輸出）。

## 4. 處理流程

### 4.1 建立校正（`POST /api/v1/meetings/{id}/corrections`）

1. 驗證：owner、`status=completed`、有音檔、`attemptsUsed < 2`、無進行中校正。
2. 單一 transaction（application 層 `WithTx`）：寫快照 + insert `queued`（attempt = 已有筆數 + 1）。
3. Enqueue 校正佇列；回傳更新後的 `correction`。

### 4.2 Worker（`CorrectionUC.Execute(correctionID)`，冪等）

1. 讀 correction；`completed` / `failed` / `reverted` → 直接結束。`queued` → `running` 並發布事件。
2. 下載音檔（GCS）→ `CompressToMP3`（16kbps mono；token 依秒計費，壓縮只為加速上傳）。
3. **切段**：依 segment 時間累積至約 12 分鐘為一段，切點對齊 segment 邊界。每段音訊 = 該段第一個 segment 起點往前 30 秒 ～ 最後一個 segment 終點（ffmpeg `-ss/-to`）。
4. **逐段依序處理**，`progress.chunks` 已有者跳過：
   - 輸入：音訊片段 + 上一段最後 3 個 segment（已套用該段 patch，唯讀上下文）+ 本段 segments（`i`、講者、時間、`AnalysisText()`、`locked = manuallyEdited`）。
   - 輸出 JSON：`[{"i": <int>, "text": "<修正後>"}]`，只含有修改的段落。
   - 驗證丟棄：`i` 不在本段、`locked`、空字串、與原文相同、長度比 < 0.5 或 > 2.0。
   - 每段完成即寫入 `progress`（斷點續跑，不重複扣費）。
5. **全部段完成後**（單一 transaction 內重讀最新 meeting）：
   - 套用 patch：跳過此刻已 `manuallyEdited` 的段（校正期間使用者的編輯優先）；`originalText` 為空才寫入（保留最初原文）；更新 `Text`、清 `MarkedText`；重新攤平 `transcript`。
   - 存逐字稿。
6. 重產摘要區塊（既有 `Summarizer`；回傳的 tags 忽略，D2a）。
7. 行動項合併（§4.3）+ 一句話摘要更新。
8. 重建該場 embedding 索引（既有 indexer）。
9. `completed`、`completed_at`，發布事件。

失敗：沿用記憶體佇列 backoff retry；達上限標 `failed` + `error_message`，發布事件。`failed` 仍佔用一次額度（D1）。新增 correction sweeper 定期把 `queued/running` 重新入列（單 instance 重啟復原，ADR-010）。

### 4.3 行動項合併

1. 分類：`source='manual'` 或 `done` 或 `edited` → **保留**；其餘 `source='llm'` → **可覆蓋**。
2. `Extractor.Extract` 以校正後逐字稿 + 保留項清單（prompt：「以下已存在，不得重複輸出」）。
3. 單一 transaction：刪可覆蓋項 → 插入新項（正規化描述後與保留項完全相同者跳過）→ sort_order：保留項原順序在前，新項接後。

### 4.4 還原（`POST /api/v1/meetings/{id}/corrections/revert`）

- 驗證最新一筆為 `completed`；單一 transaction 寫回 segments / transcript / summary_sections / summary，status → `reverted`、`reverted_at`。
- 重建 embedding 索引；行動項不動（D2）。

### 4.5 事件通知

`StatusEvent` 加 `CorrectionStatus`（omitempty）；校正各狀態變更時發布（`Status` 維持 `completed`）。前端收到該 meeting 事件即重抓詳情（沿用既有 resync）。

## 5. 程式結構（依 CLAUDE.md 新增功能流程）

| 層 | 新增 / 修改 |
|----|------------|
| `api/openapi.yaml` | 三個 endpoint（GET/POST correction、POST revert）+ MeetingDetail / TranscriptSegment 擴充 |
| `pkg/consts/errcode/` | `CorrectionLimitReached`、`CorrectionInProgress`、`CorrectionNotAllowed`（未完成/無音檔）、`NothingToRevert` |
| `domain/correction/` | `Correction` entity、`Status`、`Chunk`、`Patch`、port：`Repository`、`Corrector`（音訊 + segments → patches）、`Queue` |
| `domain/meeting/stt.go` | `TranscriptSegment` 加兩欄位；`UncertainRatio(segs)` |
| `infrastructure/db/` | correction repo、migration 000017、sqlc query |
| `infrastructure/llm/` | `CorrectChunk`（Files API 上傳片段音訊，邏輯抽自 `stt/gemini.go`）+ `prompts/correct_transcript.md` |
| `infrastructure/audio/` 或 `stt/` | `CutClip(ctx, path, from, to)`（ffmpeg） |
| `application/correction/` | `StartUC`、`ExecuteUC`、`RevertUC`；純函式 `planChunks`、`validatePatches`、`applyPatches`、`mergeActionItems` |
| `application/meeting/edit_transcript.go` | 設 `manuallyEdited=true` |
| `application/actionitem/edit.go` | 設 `edited=true` |
| `worker/` | correction queue 啟動 + sweeper |
| `interface/http/handler/correction/` + route | 兩個 handler，回傳走 `response` package |
| 前端 | 重生 TS client；`CorrectionPanel`（按鈕、次數、建議提示、進行中/失敗狀態、還原、舊資料提醒文案）；`TranscriptEditor` 段落「AI 已校正」標籤 + 原文 Sheet |

## 6. 錯誤處理

| 情境 | 行為 |
|------|------|
| 已用 2 次 / 進行中 / 未完成 | 409 / 409 / 422，errcode 對應前端文案 |
| Gemini 回非 JSON 或整段被驗證丟光 | 該段視為「無修改」記入 progress，不中斷整體 |
| Gemini / GCS / ffmpeg 錯誤 | 佇列 retry；上限後 `failed`，前端顯示可讀訊息（不暴露 cause） |
| 校正期間使用者改了段落 | 該段不套用 patch |
| 校正期間刪除紀錄 | FK cascade 刪除 correction；worker 讀不到即結束 |

## 7. 測試

- **單元（mock port，無外部依賴）**：`planChunks`（邊界、短於一段、單一超長 segment）、`validatePatches`（越界/鎖定/長度比/相同文字）、`applyPatches`（originalText 只寫一次、manuallyEdited 優先、MarkedText 清除）、`mergeActionItems`（四類保留規則、去重、排序）、`UncertainRatio`、StartUC 額度與狀態檢查、ExecuteUC 斷點續跑（progress 已有段不呼叫 Corrector）、RevertUC 只還原最新 completed 且不動行動項。
- **整合（`//go:build integration`）**：migration + repo round-trip；ffmpeg `CutClip`。
- **前端（Vitest）**：`CorrectionPanel` 各狀態渲染、建議提示門檻、還原按鈕可用條件；「AI 已校正」標籤與原文 Sheet。
- **手動 e2e**：以 Phase 26 的加噪音測試音檔與一場真實長會議（≥ 40 分鐘）驗證：錯字修正、段落銜接、計次、還原、校正中手動編輯優先、行動項保留規則。
