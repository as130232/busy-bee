# Busy Bee 產品需求

> 最後更新：2026-07-19

---

## 專案目標

Busy Bee 讓開發團隊把會議錄音自動轉成結構化技術文件（PRD 與 Tech Spec 草稿），省去人工整理會議結論與撰寫文件初稿的時間。

---

## 使用者角色

| 角色 | 場景 | 權限 |
|------|------|------|
| 團隊成員（開發者 / 技術主管） | 錄製或上傳會議音訊、閱讀生成文件、搜尋歷史會議、接收會議提醒 | Firebase Google 登入；僅能存取自己的會議與文件 |

---

## 功能範圍

### 包含（做）

- F-AUTH：Google 登入與用戶同步
- F-RECORD：瀏覽器錄音
- F-UPLOAD：音訊上傳
- F-PIPELINE：非同步音訊處理管線（STT）
- F-DOCGEN：AI 文件生成（PRD / Tech Spec）
- F-STATUS：處理狀態即時通知
- F-HISTORY：會議歷史與詳情
- F-SEARCH：關鍵字搜尋
- F-REMIND：會議行程提醒

### 不包含（不做）

- 公開註冊 / 計費 / 配額：定位為固定團隊內部工具（**從未包含**）
- 團隊共享與協作（檢視他人會議）：MVP 僅本人可見，post-MVP 再評估（**從未包含**）
- 即時逐字稿 streaming 顯示：處理完成後才顯示；WebSocket 架構已預留升級空間（**從未包含**）
- 語意搜尋（RAG / pgvector）：post-MVP 升級路徑，見 ARCHITECTURE.md ADR-006（**從未包含**）
- 逐字稿人工編輯、說話者分離（diarization）（**從未包含**）
- 管理後台（**從未包含**）

---

## Milestone 定義

| Milestone | 包含功能 | 上線目標 |
|-----------|---------|---------|
| M1-A | F-AUTH | 地基：可登入、看到空 Dashboard、CI/CD 跑通 |
| M1-B | F-RECORD、F-UPLOAD、F-PIPELINE、F-STATUS | 音訊管線跑通：上傳後即時看到轉錄完成 |
| M2-A | F-DOCGEN、F-HISTORY、F-SEARCH | MVP 上線：核心價值（會議 → 文件）可用 |
| M2-B | F-REMIND | 提醒可用 + production 完善 |

---

## 功能列表與驗收條件

### F-AUTH：Google 登入與用戶同步

**描述**：使用 Google 帳號登入，前端登入後將用戶資料同步至後端資料庫。

**前置功能**：無

**驗收條件**：
- 功能性：
  - [x] 使用 Google 帳號可完成登入並進入 Dashboard（Phase 3, `f9d77f4`，人工驗收）
  - [x] 首次登入後 users 表有對應記錄，firebase_uid 唯一（Phase 2, `4a0c408`）
  - [x] 重複登入不建立重複用戶（upsert）（Phase 2, `fa9237f`，整合測試）
- 錯誤處理：
  - [x] 未帶有效 Firebase JWT 的 API 請求回 401（Phase 2, `460edd7`，測試）
  - [x] JWT 過期時回 401，前端引導重新登入（Phase 2/3, `460edd7`）
  - [x] 非白名單 email 的帳號呼叫 /users/sync 回 403，前端顯示「無使用權限」（Phase 2, `460edd7`，測試）
- 相容性：
  - [x] 白名單以環境變數管理（email 清單），變更後重新部署即生效（Phase 2, `460edd7`）

**邊界與限制**：
- 不支援 Google 以外的登入方式
- 白名單只擋 API 層（/users/sync 與 auth middleware），不客製 Firebase 登入畫面

---

### F-RECORD：瀏覽器錄音

**描述**：PWA 內以 MediaRecorder 錄音，結束後轉入上傳流程。

**前置功能**：F-AUTH、F-UPLOAD

**驗收條件**：
- 功能性：
  - [x] 錄音可開始 / 暫停 / 結束，結束後產生音訊檔並自動進入上傳流程（Phase 8, `cc151fc`，人工驗收）
- 錯誤處理：
  - [ ] 錄音進行中離開或重新整理頁面時，顯示資料遺失警告
  - [ ] 麥克風權限被拒時顯示明確指引
- 相容性：
  - [x] Chrome 桌面版可完成錄音（webm/opus）（Phase 8, `cc151fc`，人工驗收）
  - [ ] Safari（桌面 / iOS）可完成錄音（mp4/aac）
  - [ ] 不支援的瀏覽器顯示明確錯誤訊息

**邊界與限制**：
- 不做錄音中的本地草稿保存
- 不做錄音檔剪輯

---

### F-UPLOAD：音訊上傳

**描述**：拖曳或選檔上傳預錄音訊，直傳雲端儲存（不經後端）。

**前置功能**：F-AUTH

**驗收條件**：
- 功能性：
  - [x] 支援 .mp3 / .m4a / .webm / .wav 上傳（Phase 5, `30197a3`）
  - [x] 200MB 以內檔案可完成上傳（Phase 5, `a0876cf`，人工驗收）
  - [x] 上傳完成後 meeting 進入 pending 狀態（Phase 5, `30197a3`，人工驗收）
- 錯誤處理：
  - [x] 超過 200MB 被拒且顯示明確訊息（Phase 5, `cb5496b`，GCS 整合測試驗證大小上限）
  - [x] 非音訊 content-type 被上傳端點拒絕（Phase 5, `30197a3`，測試）
  - [ ] 上傳中斷後可重試（重新取得上傳連結）

**邊界與限制**：
- 不做斷點續傳
- 不做多檔批次上傳

---

### F-PIPELINE：非同步音訊處理管線（STT）

**描述**：上傳完成後，背景任務下載音訊並轉成逐字稿（transcript），狀態機逐步推進。

**前置功能**：F-UPLOAD

**驗收條件**：
- 功能性：
  - [x] 上傳完成後狀態依序推進：pending → transcribing →（後續階段）（Phase 6/7, `275ea01`，人工驗收（WS 即時））
  - [x] 90 分鐘內的中文 / 中英夾雜音訊可產出 transcript（Phase 6, `b8608e7`，真實音訊 e2e）
  - [x] 超過 STT 服務大小上限的檔案自動壓縮後轉錄（Phase 6, `b8608e7`，ffmpeg 測試）
- 錯誤處理：
  - [x] STT 失敗自動 retry，達上限後標 failed 並記錄 error_message（Phase 6R, `b64bcb2`，測試）
  - [x] retry 不重複執行已完成的階段（冪等）（Phase 6, `7368987`，測試）

**邊界與限制**：
- 不做說話者分離（diarization）
- transcript 不提供人工編輯

---

### F-DOCGEN：AI 文件生成

**描述**：transcript 交給 LLM，生成 PRD 與 Tech Spec 兩份結構化 Markdown 文件。

**章節骨架（Q2 已確認）**：

PRD 必須包含以下章節（會議未提及的章節保留標題並標「會議未討論」）：
1. 背景與問題 — 這場會議要解決什麼
2. 目標與非目標 — 明確的做 / 不做邊界
3. 使用者與場景
4. 功能需求 — 條列，含會議中提到的優先序
5. 驗收條件 — 可 yes/no 判斷
6. 開放問題 — 會議中未決的事項
7. 決議與行動項 — 含負責人與時限（若會議有提及）

Tech Spec 必須包含：
1. 背景與現況
2. 技術方案概述 — 含會議中討論過的替代方案與取捨
3. 架構與流程設計
4. 資料模型 / API 影響
5. 風險與緩解
6. 待確認技術問題
7. 建議實作步驟

**前置功能**：F-PIPELINE

**驗收條件**：
- 功能性：
  - [x] 每次成功處理產生恰好兩份 artifacts（prd、tech_spec）（Phase 9, `cfd2833`，e2e）
  - [x] 產出為結構化 Markdown，含固定章節骨架（Phase 9, `cfd2833`，e2e）
  - [x] 中文會議產出中文文件（Phase 9, `cfd2833`，e2e）
- 錯誤處理：
  - [x] LLM 失敗自動 retry，達上限後標 failed（Phase 9, `cfd2833`，測試）
  - [x] retry 不產生重複 artifacts（冪等）（Phase 9, `369234b`，UNIQUE upsert + 測試）
  - [x] transcript 內容不足以填滿某章節時，保留章節標題並標「會議未討論」，禁止捏造內容（Phase 9, `cfd2833`，e2e 實見「會議未討論」）

**邊界與限制**：
- 章節骨架固定如上，不做用戶自訂模板（post-MVP）

---

### F-STATUS：處理狀態即時通知

**描述**：透過 WebSocket 即時推送 meeting 狀態變更給前端。

**前置功能**：F-AUTH

**驗收條件**：
- 功能性：
  - [x] 狀態變更後前端 3 秒內收到更新（正常網路）（Phase 7, `60e25d8`，人工驗收）
  - [x] 斷線後自動重連，重連後補齊到最新狀態（Phase 7, `60e25d8`，退避重連實作）
  - [x] 只收到自己 meeting 的通知（Phase 7, `275ea01`，測試）
- 錯誤處理：
  - [x] 未通過 JWT 驗證的 WebSocket 連線收不到任何資料（Phase 7, `275ea01`，測試）

**邊界與限制**：
- 不推送逐字稿內容（僅狀態事件）

---

### F-HISTORY：會議歷史與詳情

**描述**：會議列表與詳情頁，檢視 transcript 與生成文件。

**前置功能**：F-PIPELINE

**驗收條件**：
- 功能性：
  - [x] 列表按建立時間倒序顯示本人全部會議（Phase 10, `5eeb122`，整合測試 + 人工）
  - [x] 詳情頁顯示 transcript 與兩份文件（Markdown 渲染）（Phase 10, `16fcd89`，人工驗收）
  - [x] 只顯示本人的會議（Phase 10, `5eeb122`，跨用戶整合測試）
- 錯誤處理：
  - [x] failed 的會議顯示錯誤原因與重試按鈕（Phase 10, `5eeb122, 16fcd89`，retry UC 測試）

**邊界與限制**：
- 不做文件匯出（post-MVP）

---

### F-SEARCH：關鍵字搜尋

**描述**：以關鍵字搜尋會議 title 與 transcript。

**前置功能**：F-HISTORY

**驗收條件**：
- 功能性：
  - [x] 關鍵字命中 title 或 transcript 即出現在結果（Phase 10, `5eeb122`，整合測試 + 人工）
  - [x] 只搜尋本人的會議（Phase 10, `5eeb122`，整合測試）
- 錯誤處理：
  - [x] 空關鍵字回傳一般列表，不報錯（Phase 10, `5eeb122`，測試）

**邊界與限制**：
- 不做語意搜尋（post-MVP，見 ADR-006）

---

### F-REMIND：會議行程提醒

**描述**：建立含排定時間的未來會議，於會議前推播提醒。

**前置功能**：F-AUTH

**驗收條件**：
- 功能性：
  - [x] 可建立 / 編輯含 scheduled_at 的未來會議（Phase 11, `bb48b3a`，人工建立驗證）
  - [x] 每場會議可設定提醒提前時間，未設定時預設 15 分鐘（Phase 11, `bb48b3a`，測試 + 人工（1 分鐘））
  - [ ] 於 scheduled_at 前的設定時間收到推播
  - [x] 修改 scheduled_at 後舊提醒不觸發、新提醒生效（Phase 11, `bb48b3a`，UpdateSchedule 清 reminded_at，測試）
  - [ ] 取消推播訂閱後不再收到推播
- 相容性：
  - [ ] iOS 16.4+ 且 PWA 已加入主畫面時可收到推播（平台限制，載明於 UI）

**邊界與限制**：
- 不做行事曆（Google Calendar 等）整合

---

## 優先序

| 優先級 | 功能 | 說明 |
|--------|------|------|
| P0.1 MVP | F-AUTH | 一切功能的前提 |
| P0.2 MVP | F-UPLOAD | 音訊進入系統的入口 |
| P0.3 MVP | F-PIPELINE | 核心管線 |
| P0.4 MVP | F-DOCGEN | 核心價值：會議 → 文件 |
| P0.5 MVP | F-STATUS | 非同步流程的必要回饋 |
| P0.6 MVP | F-RECORD | 錄音入口（可先用上傳頂替） |
| P1.1 重要 | F-HISTORY | 歷史檢視 |
| P1.2 重要 | F-SEARCH | 快速找回內容 |
| P2.1 之後 | F-REMIND | 加分功能 |

---

## 非功能性需求

| 類別 | 要求 |
|------|------|
| 效能 | 音訊上傳一律直傳雲端儲存（不經後端）；90 分鐘音訊完整處理（STT + 文件生成）必須在 15 分鐘內完成 |
| 可用性 | 後端 scale-to-zero（冷啟動數秒可接受）；未完成處理由復原掃描續跑；內部工具，無正式 SLA |
| 安全 | 所有 API 必須帶有效 Firebase JWT；資料僅本人可見（一律以 user_id 過濾）；外部錯誤原始 cause 不回傳用戶端；不得 log 用戶 token；API keys 一律存 Secret Manager |
| 可觀測性 | 一律使用結構化 log（slog）；處理管線每次狀態變更必須留有 log（含 meeting_id）；HTTP 4xx / 5xx 必須帶 traceId |
| 成本 | 基礎設施月費 ≤ USD 25（不含 STT / LLM 用量費） |
| 相容性 | Chrome 與 Safari（桌面 + iOS）必須可錄音與上傳；iOS 推播限制必須於 UI 載明 |

---

## 待確認事項

| 編號 | 問題 | 影響範圍 | 確認對象 | 期限 |
|------|------|---------|---------|------|
| Q1 | 登入是否限制白名單（任何 Google 帳號皆可 vs 僅指定成員）？ 答：採 email 白名單（環境變數管理），非名單成員拒絕（2026-07-17 確認） | F-AUTH 權限設計 | Charles | M1-A 開發前 |
| Q2 | PRD / Tech Spec 的章節模板內容？ 答：採本文件 F-DOCGEN 所列章節骨架（2026-07-17 確認） | F-DOCGEN prompt 設計 | Charles | M2-A 開發前 |
| Q3 | 提醒提前時間固定 15 分鐘，或可由用戶設定？ 答：可逐場會議設定，預設 15 分鐘（2026-07-17 確認） | F-REMIND | Charles | M2-B 開發前 |
