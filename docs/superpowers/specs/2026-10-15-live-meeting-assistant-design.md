# 即時會議助理（F-LIVE）設計

- 日期：2026-10-15
- 狀態：設計確認，待實作（Phase 30）
- 範圍：子專案 A「即時會議基礎」。子專案 B「面試助理」（履歷、即時建議回答、中英翻譯）在 A 驗收後另開 spec，複用本 spec 的會前資料與即時問答機制。

## 1. 目標與成功標準

**目標**：在 Google Meet 網頁版開會時，busy-bee 同時擷取 Meet 分頁音訊（遠端講者）與麥克風（自己），會議中即時顯示逐字稿、定期整理重點，並可針對本場內容提問；會議結束後照既有管線產生正式紀錄。

**成功標準**
- 對方講話後約 1 秒內出現即時逐字稿；重點約每 1.5 分鐘更新一次。
- 會議中提問能依「本場逐字稿＋會前資料」回答並標示來源，查無依據時明說，不編造。
- 會議結束後自動走現有管線（整批 STT、分講者、摘要、行動項），正式紀錄品質不低於一般錄音上傳。
- busy-bee 分頁意外關閉，錄音可救回並上傳。
- 每月新增費用可預測：預設設定（省錢版）一天 1～2 場、每場 30～60 分鐘，約 US$14／月（2026-10-15 牌價，見 §9）。
- 後端維持 scale-to-zero：會議中後端只在呼叫 Gemini 時喚醒。

**不做（YAGNI）**
- 面試助理（子專案 B）。
- Teams／Zoom 桌面版音訊（只預留 `AudioSource` 擴充點，見 §4.1）。
- 會議中即時抽取行動項（會後既有流程處理）。
- 會前資料庫（跨會議重複選用）：v1 每場各自上傳（D6）。
- 保存即時逐字稿：以正式版為準（D4）。
- 手機或 Safari 上的即時模式：`getDisplayMedia` 分頁音訊只有桌面版 Chrome 支援。

## 2. 已確認的產品決策

| # | 決策 |
|---|------|
| D1 | 先做 Google Meet 網頁版（Chrome 分頁音訊＋麥克風）；Teams 之後再做，現在只預留音訊來源抽象層 |
| D2 | 第一版功能：A 跟上進度（即時重點）、C 回頭查原話（即時逐字稿）、D 會議中提問。不做 B 即時待辦 |
| D3 | 語言：中文為主，夾英文術語 → 依會議 `language`（預設 zh-TW）＋術語加權 |
| D4 | 即時逐字稿不保存；會後以錄音重跑既有管線產生的正式版為準。例外：上傳完成前，即時逐字稿暫存在本機，作為上傳失敗時的備份 |
| D5 | 會議中的「即時重點」最終版與「提問紀錄」結束時存進該紀錄，詳情頁以「會議中筆記」顯示（可收合） |
| D6 | 會前資料：可貼文字或上傳 `.txt` / `.md` / `.pdf`，每場各自上傳（不做資料庫）；也可事先掛在排程紀錄上 |
| D7 | 會前資料用於：即時重點、會議中提問（標來源）、自動抽專有名詞給 Deepgram 術語加權、會後正式摘要／行動項／AI 校正 |
| D8 | 費用配置 = C：預設省錢版（只即時辨識 Meet 分頁、即時不分講者），設定頁開關「即時辨識我的麥克風」打開即為完整版 |
| D9 | 每月即時分鐘數上限（環境變數）；超過時只停即時辨識，錄音照常 |
| D10 | 錄音每 10 秒寫入 IndexedDB；分頁意外關閉後重新開啟可救回並上傳 |
| D11 | AI 校正維持手動觸發（不自動執行），校正時一併帶入會前資料 |
| D12 | 架構採方案 1：瀏覽器以短效金鑰直連 Deepgram；後端只負責 Gemini 與狀態，不保存會議中狀態 |

## 3. 架構總覽

```
Chrome（busy-bee 分頁）
 ├─ AudioSource（MeetTabSource：getDisplayMedia 分頁音訊 + getUserMedia 麥克風）
 │    ├─ 分頁音訊 ──► LiveTranscriber ──WS(短效 JWT)──► Deepgram 串流 ──► 即時逐字稿（對方）
 │    ├─ 麥克風   ──► LiveTranscriber（開關打開時）────► Deepgram 串流 ──► 即時逐字稿（我）
 │    └─ 混音（AudioContext）──► MediaRecorder 32 kbps ──► IndexedDB（每 10 秒）
 ├─ NotesScheduler（每 ~90 秒 / 手動）──HTTP──► 後端 /live/notes ──► Gemini
 ├─ 提問 ──HTTP(SSE)──► 後端 /live/ask ──► Gemini（串流回答）
 └─ 結束 ──► /live/finish（存筆記、轉狀態、取得上傳 URL）──► GCS 直傳 ──► complete-upload ──► 既有管線
```

後端不保存會議中的狀態：即時逐字稿、目前重點都在瀏覽器記憶體（＋IndexedDB 備份），每次呼叫時帶入。

## 4. 前端設計

### 4.1 音訊來源抽象（Teams 擴充點）

```ts
interface AudioSource {
  start(): Promise<void>        // 取得權限、建立 MediaStream
  remote: MediaStream           // 遠端講者音訊（Meet 分頁）
  local: MediaStream | null     // 自己的麥克風
  onEnded(cb: () => void): void // 使用者停止分享或來源關閉
  stop(): void
}
```

v1 只實作 `MeetTabSource`。之後加 Teams 桌面版時，新增 `VirtualDeviceSource`（例如 BlackHole 裝置），下游的轉文字、重點、提問、錄音都不用改。

`MeetTabSource` 規則：
- `getDisplayMedia({ video: true, audio: true, preferCurrentTab: false, selfBrowserSurface: 'exclude' })`。取得後立刻停掉 video track，只保留音訊。
- 沒有 audio track（沒勾「分享分頁音訊」）時拋出明確錯誤，UI 提示重選。
- 麥克風一律取得，因為錄音需要；設定開關只決定要不要送給 Deepgram。
- 分頁音訊 track `ended` 時觸發 `onEnded`，進入結束流程。

### 4.2 LiveTranscriber（Deepgram 串流 client）

- 連線：`wss://api.deepgram.com/v1/listen?…`，以 `Sec-WebSocket-Protocol: ['bearer', <jwt>]` 帶短效金鑰（Task 0 驗證）。
- 參數由後端 `/live/token` 一併回傳，前端不寫死。內容：`model`、`language`、`interim_results=true`、`smart_format=true`、`punctuate=true`、`endpointing=300`、`utterance_end_ms=1000`、`keyterm`（全域術語＋會前資料抽出的術語）、`diarize=false`。
- 音訊：每路一個 `MediaRecorder(webm/opus)`，`timeslice` 250ms，送出 Blob。
- KeepAlive：連續 5 秒沒有送音訊時，送 `{"type":"KeepAlive"}`。
- 斷線重連：
  1. 指數退避（1s、2s、4s，上限 10s）。
  2. 每次重連都向後端取新金鑰。
  3. 重連期間的音訊 Blob 暫存在佇列（上限 30 秒），重連後依序補送。
  4. 超過 30 秒的部分丟棄，逐字稿插入「（斷線 N 秒，正式版會補上）」。
- 結果：`is_final=true` 的結果寫入逐字稿；interim 只顯示在最後一行的灰字。每行記錄相對時間與來源（`remote` 或 `local`）。
- 中文簡轉繁：live 固定 `language` 不使用 detect，所以不需要（ADR-014 只有 auto 時才轉）。

### 4.3 NotesScheduler（即時重點）

- 每 90 秒觸發一次，條件是自上次更新後新增的 final 逐字稿 ≥ 200 字；不滿就跳過這一輪。
- 也可手動按「立即更新」，不受字數門檻限制，但兩次至少間隔 15 秒。
- 請求內容：`{ notes: 目前重點, newTranscript: 上次之後的新逐字稿 }`。回應：新版重點。
- 失敗時保留舊重點，顯示「更新失敗，稍後重試」，下一輪照常。
- 同一時間只有一個請求在跑；前一個還沒回來時，這一輪跳過。

重點的資料格式：

```json
{ "current": "目前在討論：…（一句話）", "points": ["已談到的重點 1", "…"] }
```

### 4.4 會議中提問

- 請求：`{ question, transcript: 完整即時逐字稿（含時間與來源）, history: 本場前幾輪問答（最多 5 輪） }`。
- 回應：SSE 串流文字，最後一則事件附上來源清單：`[{ "type": "transcript", "at": "14:03" } | { "type": "context", "title": "規格書.pdf" }]`。
- 問答紀錄存在瀏覽器記憶體，結束時存進紀錄（D5）。

### 4.5 錄音與救回（D10）

- 分頁與麥克風用 `AudioContext` 混音，接到 `MediaStreamDestination`，再交給 `MediaRecorder`。沿用 `AUDIO_BITS_PER_SECOND = 32_000`，`timeslice` 10 秒。
- 每個 chunk 寫入 IndexedDB 的 `live-recordings` store，key 為 `meetingId`，值為 chunk 陣列，另存標題、開始時間、即時筆記與提問紀錄的快照。
- 上傳成功並 `complete-upload` 之後，刪除該筆 IndexedDB 資料。
- App 啟動時檢查 IndexedDB 是否有殘留資料。有的話跳出提示：「有一場未完成的即時會議（標題、時間），要上傳嗎？」
  - 選「上傳」：走 `/live/finish` → 上傳 → `complete-upload`。
  - 選「捨棄」：刪除本機資料，並呼叫 `DELETE /meetings/:id`。
- IndexedDB 寫入失敗（例如空間不足或無痕模式）時不中斷錄音，只顯示一次警告「本機備份不可用，請勿關閉分頁」。

### 4.6 畫面

- 錄音頁新增「即時會議」入口。只在支援 `getDisplayMedia` 音訊的桌面瀏覽器顯示；其他環境顯示為不可用並說明原因。
- 開始前：選情境、確認標題（預設「會議 YYYY-MM-DD HH:mm」，使用本地時間）、選擇或貼上會前資料（可略過）。
- 開始順序：
  1. 取得音訊。
  2. `POST /meetings/live` 建立紀錄。
  3. 立即以全域術語開始即時辨識，不等會前資料。
  4. 同時在背景上傳並轉換會前資料。轉換完成後：
     - 之後的重點與提問自動帶入資料。
     - 以新術語重開一次 Deepgram 連線，套用 §4.2 的補送機制，不掉字。
- 想讓資料一開始就生效，可以先建立排程、提前上傳資料，再從排程開始（`scheduledMeetingId`）。
- 會議中：
  - **寬版：** 左邊是「即時重點」，右邊是「即時逐字稿」，底部是提問框。
  - **窄版：** 以頁籤切換「重點／逐字稿／提問」。
  - **狀態列：** 經過時間、連線狀態（正常／重連中／即時辨識已停用）、本月已用分鐘數、「結束」按鈕。
- 逐字稿捲動：新內容自動捲到底；使用者往上捲時暫停自動捲動，並出現「回到最新」按鈕。
- 結束：先確認，再顯示上傳進度，完成後導到詳情頁（處理中狀態沿用既有 F-STATUS）。
- 詳情頁：新增可收合的「會議中筆記」區塊（最後一版重點＋問答紀錄）與「會前資料」清單（可新增、刪除）。
- 設定頁：新增「即時辨識我的麥克風」開關（存在使用者偏好，見 §5.1）。

## 5. 後端設計

### 5.1 資料結構（migration 000018）

**meetings 狀態新增 `live`**：更新 `meetings_status_check`。

狀態轉換新增：

```
（新建）→ live
scheduled → live   （從排程開始即時會議）
live → scheduled   （結束：存筆記、等待上傳音訊；沿用「scheduled = 待上傳音訊」既有語意）
```

`scheduled → pending` 之後的轉換完全不變。

**meetings 新欄位：**

| 欄位 | 型別 | 說明 |
|------|------|------|
| `live_notes` | jsonb NULL | `{ "notes": {current, points}, "qa": [{question, answer, sources, askedAt}] }` |
| `live_seconds` | int NOT NULL DEFAULT 0 | 本場即時辨識用掉的秒數（心跳累加，用於月上限） |
| `live_started_at` | timestamptz NULL | 開始即時會議時間 |

**新表 `meeting_contexts`：**

| 欄位 | 型別 | 說明 |
|------|------|------|
| `id` | uuid PK | |
| `meeting_id` | uuid FK → meetings ON DELETE CASCADE | |
| `user_id` | uuid FK → users ON DELETE CASCADE | 所有查詢以 user_id 過濾 |
| `title` | text | 檔名，或使用者為貼上文字取的名稱（預設「貼上的文字」） |
| `source` | text | `text` / `file`（CHECK） |
| `gcs_path` | text NOT NULL DEFAULT '' | 原始檔（僅 file） |
| `content` | text | 轉換後的純文字（Gemini 轉換或直接貼上） |
| `keyterms` | jsonb | 抽出的專有名詞陣列（≤ 50 個） |
| `created_at` | timestamptz | |

**使用者偏好**：`users` 新增 `live_mic_transcription boolean NOT NULL DEFAULT false`。

### 5.2 端點（OpenAPI first）

| 方法 | 路徑 | 說明 |
|---|---|---|
| POST | `/meetings/live` | `{title, scenario, language, scheduledMeetingId?}` → 建立 `live` 紀錄；有 `scheduledMeetingId` 時，把該排程轉為 `live` |
| GET / POST | `/meetings/:id/contexts` | 列出／新增會前資料。新增時，JSON `{title, text}` 或 multipart 檔案（`.txt` `.md` `.pdf`，≤ 10MB） |
| DELETE | `/meetings/:id/contexts/:contextId` | 刪除會前資料（含 GCS 檔） |
| POST | `/meetings/:id/live/token` | 檢查狀態為 `live` 且未超過月上限 → 回 `{accessToken, expiresIn, params}` |
| POST | `/meetings/:id/live/heartbeat` | `{seconds}`（距上次心跳的即時辨識秒數）→ 累加 `live_seconds`，回 `{usedMinutesThisMonth, limitMinutes, overLimit}` |
| POST | `/meetings/:id/live/notes` | `{notes, newTranscript}` → 新版重點 |
| POST | `/meetings/:id/live/ask` | `{question, transcript, history}` → SSE 串流回答＋來源 |
| POST | `/meetings/:id/live/finish` | `{notes, qa, contentType}` → 存 `live_notes`，`live → scheduled`，回上傳 URL（同 `POST /meetings` 的 `UploadTarget`） |
| PATCH | `/users/me/preferences` | `{liveMicTranscription}` |

上傳完成後沿用既有 `POST /meetings/:id/complete-upload`（`scheduled → pending`）。

所有端點以 `user_id` 過濾，錯誤碼新增於 `pkg/consts/errcode/`：

| 錯誤碼 | 何時回傳 |
|---|---|
| `LiveNotActive` | 狀態不是 `live` |
| `LiveQuotaExceeded` | 超過每月上限 |
| `ContextTooLarge` | 會前資料超過大小上限 |
| `ContextUnsupportedType` | 不支援的檔案格式 |
| `ContextExtractFailed` | 檔案轉文字失敗 |

### 5.3 分層

- `domain/live/`：
  - `Notes`、`QAEntry`、`Source` entity。
  - port：`TokenIssuer`（Deepgram grant）、`LiveAssistant`（`UpdateNotes`、`Ask` 串流）。
  - 用量查詢 port。
- `domain/meetingcontext/`：
  - `Context` entity。
  - `Repository`。
  - `Extractor` port（檔案轉文字並抽術語）。
- `infrastructure/stt/deepgram_grant.go`：呼叫 `POST /v1/auth/grant`，TTL 30 秒。
- `infrastructure/llm/`：
  - `live.go`：`UpdateNotes`、`Ask`（`GenerateContentStream`）。
  - `context_extract.go`：PDF 以 inline data 交給 Gemini 轉 markdown 並抽術語；txt/md 直接讀取，只呼叫一次 Gemini 抽術語。
  - prompt 檔：`prompts/live_notes.md`、`prompts/live_ask.md`、`prompts/context_extract.md`。
- `application/live/`：
  - `StartUC`、`IssueTokenUC`（含月上限檢查）、`HeartbeatUC`、`NotesUC`、`AskUC`、`FinishUC`。
  - `NotesUC` 與 `AskUC` 從 `meetingcontext.Repository` 讀會前資料並組 prompt。
- `application/meetingcontext/`：`AddUC`、`ListUC`、`DeleteUC`。
- `interface/http/handler/live/`、`handler/meetingcontext/`。SSE 回應透過 `response` package 新增的串流 helper，不直接寫 `c.JSON`。

### 5.4 會前資料帶入既有流程（D7）

- **prompt 組裝：** 新增共用函式 `meetingcontext.PromptBlock(contexts) string`。
  - 輸出 `## 背景資料（會前提供，非會議內容）` 段落，每份資料以 `### <title>` 分隔。
  - 內容一律放在 prompt 最前面，讓 Gemini 隱性快取能命中。
- **摘要、行動項、AI 校正：** `Summarize`、`ExtractExcluding`、`CorrectChunk` 新增 `background string` 參數。
  - 背景資料為空時，prompt 與現在完全相同。
  - prompt 規則：背景資料只用於理解與修正專有名詞，**摘要內容只能來自逐字稿**。
- **Deepgram 術語：** 整批 STT（`Transcribe`）和串流 token 的 `params.keyterm`，都合併「全域 `DEEPGRAM_KEYWORDS`＋該場會前資料的 keyterms」。去重後上限 100 個，超過時以全域優先。

### 5.5 Prompt 規則

**`live_notes.md`：**
- **輸入：** 背景資料、目前重點、新逐字稿。
- **輸出：** JSON `{current, points}`。
- **規則：**
  - `points` ≤ 15 條；要合併重複、保留具體數字與名稱。
  - 只根據逐字稿；背景資料只用來理解。

**`live_ask.md`：**
- 只能依據逐字稿與背景資料回答。
- 每個論點都要標來源：`〔逐字稿 HH:MM〕` 或 `〔資料：<title>〕`。
- 兩邊都沒有依據時，回答「會議中沒提到、會前資料也沒有」，不得推測。
- 以繁體中文回答。

**`context_extract.md`：**
- 把檔案轉成 markdown 純文字（保留標題、表格、條列）。
- 另外抽出 ≤ 50 個專有名詞（產品名、系統名、人名、縮寫）。

### 5.6 月用量上限（D9）

- **設定：** 環境變數 `LIVE_MONTHLY_MINUTES_LIMIT`，預設 3000。
- **計算：** 以 `SUM(live_seconds) WHERE user_id = ? AND live_started_at >= 本月第一天（Asia/Taipei）` 計算本月用量。
- **心跳：** 前端每 5 分鐘送一次心跳，`seconds` 為本段實際有連上 Deepgram 的秒數（單路計算，見 §9）。
- **超限時：**
  - `/live/token` 回 `LiveQuotaExceeded`。
  - 心跳回傳 `overLimit=true` 時，前端關閉 Deepgram 連線、停用即時重點，錄音繼續，狀態列顯示「本月即時額度已用完，錄音繼續」。
- **單次請求的大小上限：**

  | 端點 | 上限 |
  |---|---|
  | `/live/notes` 的 `newTranscript` | 20,000 字 |
  | `/live/ask` 的 `transcript` | 120,000 字（約 4 小時中文） |
  | 會前資料（單場合計） | 100,000 字 |

  上限另以環境變數 `LIVE_CONTEXT_MAX_CHARS` 等設定。超過時回 `Param` 錯誤。

- **頻率限制：** `/live/notes` 每場每 15 秒最多 1 次，`/live/ask` 每場每 5 秒最多 1 次，使用 in-process 限流（ADR-010 單 instance）。

### 5.7 卡住的 `live` 紀錄

- 列表顯示「即時會議未完成」標籤。詳情頁提供兩個選項：
  - 「上傳錄音檔」：手動選檔，走 `/live/finish` → 上傳。
  - 「刪除」。
- 不自動標成 failed，因為使用者可能稍後才從 IndexedDB 救回。

## 6. 安全與隱私

- **金鑰：**
  - Deepgram 主金鑰仍在 Secret Manager，只在後端使用。
  - 短效 JWT 的 TTL 為 30 秒，權限為 `usage::write`，無法管理帳號。
- **權限：** 所有新查詢以 `user_id` 過濾。會前資料存在 GCS 的 `contexts/<userID>/<meetingID>/<contextID><ext>`。
- **log：** 只記 `meeting_id`、`context_id`、字數、耗時。逐字稿、會前資料內容、問題與回答都不寫入 log。
- **WebSocket：** 瀏覽器直連的 Deepgram WS 不經過本站 WS hub，不受 ADR-002 影響。本站 F-STATUS 通道照舊。
- **錄音同意：** 開始畫面加一行提示：「請依公司規範告知與會者本場有錄音與 AI 整理」。

## 7. 錯誤處理

| 情況 | 處理 |
|---|---|
| 未分享分頁音訊 | 提示重選，不建立紀錄（先取得音訊，成功後才呼叫 `/meetings/live`） |
| 麥克風被拒 | 允許繼續，但提示「你的聲音不會被錄到」，錄音只包含分頁音訊 |
| Deepgram 斷線 | 依 §4.2 重連與補送；超過 30 秒顯示斷線標記；錄音不中斷 |
| 金鑰申請失敗 | 依退避重試；連續 5 次失敗就停用即時辨識，錄音繼續 |
| `/live/notes` 失敗 | 保留舊重點，下一輪重試 |
| `/live/ask` 失敗或逾時（90 秒） | 顯示錯誤，可重問 |
| 停止分享或 Meet 分頁關閉 | 自動進入結束流程（顯示確認，30 秒內沒有回應就自動結束） |
| busy-bee 分頁關閉或當掉 | IndexedDB 救回（§4.5） |
| 上傳失敗 | 沿用既有上傳重試；IndexedDB 資料保留到 `complete-upload` 成功 |
| 會前資料轉換失敗 | 回 `ContextExtractFailed`，不建立該筆資料，提示改用貼上文字 |
| 超過月上限 | 依 §5.6 處理 |

## 8. 測試

**Task 0（實作前先實測，失敗就回報再討論）**
1. 在 Chrome＋真實 Google Meet 上，`getDisplayMedia` 能取得分頁音訊，且使用者仍聽得到 Meet 聲音。
2. `/v1/auth/grant` 的 JWT 能用 `['bearer', jwt]` subprotocol 從瀏覽器連上 Deepgram 串流。
3. 設定的 `model` 與 `language=zh-TW` 在串流模式可用，延遲和準確度可接受。
   - nova-3 串流若不支援 zh-TW，改用 nova-2 串流。
   - 會後整批辨識維持現有設定。
4. SSE 經 Firebase Hosting rewrite 到 Cloud Run 能逐段送達，不會被緩衝到最後才一次送出。不行的話，`/live/ask` 改直連 Cloud Run 網址（需設定 CORS），或改為不串流。

**後端單元測試**（mock port，不連真實 API）
- 狀態轉換：`new→live`、`scheduled→live`、`live→scheduled`。其他狀態呼叫 live 端點時回 `LiveNotActive`。
- 月上限：剛好等於上限、超過上限、跨月重置（Asia/Taipei）。
- `PromptBlock`：空資料時輸出空字串；多份資料的順序與標題。
- 術語合併：去重、上限 100、全域優先。
- 會前資料：大小上限、格式白名單、轉換失敗不寫入 DB、刪除時連帶刪除 GCS。
- `FinishUC`：寫入 `live_notes`、轉換狀態、回傳上傳 URL；重複呼叫具冪等性（已是 `scheduled` 時重發上傳 URL）。
- 既有 `Summarize`、`Extract`、`Correct` 在背景資料為空時，prompt 與現在完全相同（回歸測試）。
- handler：權限（別人的 meeting 回 404）、限流。

**前端單元測試**（Vitest，假 MediaStream 與假 WebSocket）
- `MeetTabSource`：沒有 audio track 時拋錯；分頁 track `ended` 時觸發 `onEnded`。
- `LiveTranscriber`：final 與 interim 的處理、KeepAlive 計時、斷線重連（取新金鑰）、30 秒內補送、超過 30 秒插入斷線標記。
- `NotesScheduler`（fake timers）：字數門檻、手動更新的間隔、同時只有一個請求、失敗時保留舊重點。
- IndexedDB 救回：寫入、啟動時偵測、上傳成功後清除、寫入失敗時只警告。
- 畫面：往上捲時暫停自動捲動、窄版頁籤、超限時的狀態列。

**人工驗收**
- 實際開一場 30 分鐘以上的 Meet 會議（含 2 位以上發言者），檢查即時逐字稿、重點更新、提問與來源、會後正式紀錄、「會議中筆記」區塊。
- 會議中關掉 busy-bee 分頁後重新開啟，確認救回並上傳成功。
- 開啟麥克風開關再測一場，確認「我」的逐字稿出現。

## 9. 成本（2026-10-15 牌價，Deepgram 串流為限時促銷價）

| 項目 | 單價 |
|---|---|
| Deepgram nova-3 串流 | $0.0048／分 |
| keyterm | +$0.0013／分 |
| 串流 diarization | +$0.0020／分（v1 不使用） |
| Gemini 3 Flash | 輸入 $0.50／M、輸出 $3.00／M |

| 配置 | 每分鐘 | 一天 1～2 場 × 30～60 分（每月約 1,500 分鐘） |
|---|---|---|
| 省錢版：只辨識分頁＋keyterm＋重點 | 約 $0.009 | 約 $14 |
| 開啟麥克風辨識 | 約 $0.015 | 約 $23 |

- 會前資料每小時增加約 $0.3（以 20 頁 PDF 計，沒有快取時）。命中隱性快取時更低，需要實測。
- 月上限的分鐘數以會議時長計算，不論開了幾路辨識。開麥克風時實際費用約為 1.6 倍，上限的預設值已經把這點算進去。

## 10. 文件更新（實作時）

- `docs/PRODUCT.md`：
  - 新增 F-LIVE 規格與驗收條件。
  - 「不包含」中「即時逐字稿 streaming 顯示」改為已由 F-LIVE 實作（即時逐字稿不保存）。
- `docs/ARCHITECTURE.md`：
  - 新增 ADR-018：即時會議以瀏覽器直連 Deepgram，後端無狀態，不代轉。
  - 補充狀態機與資料流。
- `docs/PLAN.md`：新增 Phase 30。
- `CLAUDE.md`：在「判斷要動哪個檔案」補一行即時會議與會前資料的位置。
