# PLAN 當前焦點歷史（封存）

> 2026-10-16 自 `docs/PLAN.md`〈當前焦點〉原文搬出（project-docs upgrade），內容未改。由新到舊排列。

**2026-10-15 Phase 30 即時會議助理（F-LIVE）程式完成（分支 `feat/live-meeting`）**：Google Meet 網頁版會議中擷取分頁＋麥克風，Deepgram 即時逐字稿、Gemini 即時重點與會議中提問、會前資料（文字/txt/md/pdf）、錄音本機備份救回；會前資料也帶入會後摘要/行動項/AI 校正（ADR-018）。後端 vet/test（含整合測試）全綠、前端 typecheck/lint/test（108）/build 全綠。**上線前需換 Deepgram Member 權限金鑰（30.9）**；剩真實 Meet 驗收（30.10）+ merge/部署。下一階段：面試助理（子專案 B）。

**2026-10-13 Phase 29 前端上線前稽核修正（分支 `fix/fe-audit`）**：修 F1–F10（錄音標題時區、分享連結需確認、上傳大小、請求逾時、登出清本機資料、縮放、還原確認、推播導向、lint），錄音改 32 kbps；F11 排程提醒決定不加 Cloud Scheduler（省成本）。前端 typecheck/lint/test（60）/build 全綠。剩 29.12 未驗證項 + 裝置端驗收 + merge/部署。

**2026-10-12 Phase 28 摘要細節化 + 模型備援**：使用者回報改用 3.5-flash-lite 後報告過度簡略；實測確認主因為 Lite 模型。改回 gemini-3-flash-preview（thinking LOW）+ 404 自動備援 gemini-3.5-flash；會議 prompt 改為保留可執行細節並新增「流程與規範細節」「待確認事項」區塊（ADR-017）。待使用者以原會議重跑 AI 校正驗收。

**2026-10-10 Phase 27 AI 錄音重新校正（F-CORRECT）程式完成（分支 `feat/ai-correction`）**：詳情頁「AI 校正」→ Gemini 聽已上傳錄音逐段修正逐字稿 → 重產摘要、合併行動項；每場 2 次、整份還原、手動修改段落不覆蓋（ADR-016）。後端 vet/test（含整合測試）全綠、前端 typecheck/lint/test/build 全綠、本地 migration 000017 已套用。剩使用者裝置端 e2e（27.12）+ merge/部署（prod migration 由 CI 自動套用）。

**2026-10-10 Phase 26 收音不佳時的報告完整度程式完成**：STT 低信心字詞標記給 LLM、prompt 加 ASR 容錯與「（語音不清，待確認）」註記、nova-3 改用 `keyterm`（ADR-015）。後端 vet/test 全綠；剩真實低品質錄音驗證（26.5）+ merge/部署。

**2026-09-19 Phase 25 per-meeting STT 語言選擇程式完成**：STT 語言從全域 config 寫死 `zh-TW` 改成每筆會議可選中文/英文/自動偵測；已用真實 Deepgram API + TTS 測試音檔實測確認 `language=multi` 對純中文音訊仍會壞掉（ADR-011 踩坑至今未修），改用 `detect_language=true` 做自動偵測（正確可用，但中文結果為簡體，已接 OpenCC `longbridgeapp/opencc` 後製轉繁體）。後端 build/vet/test 全綠（新增 domain/STT/application 層測試）、前端 typecheck/lint/test/build 全綠、DB migration 000016 已套用（本地）、sqlc/openapi/TS client 已重生成。詳見 `docs/ARCHITECTURE.md` ADR-014。剩裝置端 e2e 人工驗收（中/英/自動偵測各一段音檔）+ merge/部署。

**2026-08-04 前端首次檢驗與重構（分支 `refactor/fe-first-pass`，進行中）**：三路審查（效能/安全/品質）後分階段清技術債，效能與可維護性並重。已完成並 typecheck/lint/build 全綠：
- Phase 1（效能）：路由 `React.lazy` code splitting + `vite` manualChunks 切 firebase/markdown → 主 bundle 540KB→262KB；`MeetingDetailPage.speakerOrder` useMemo。
- Phase 2（消重複）：新增 `utils/format.ts`（formatClock/formatDuration/formatDateTime）與 `hooks/useFileUpload.ts`（RecorderPanel/UploadZone 共用上傳流程）。
- Phase 3（API 集中化）：`client.ts` `setUnauthorizedHandler` 集中 401→登出（AuthProvider 註冊）；新增 `hooks/useApiCall.ts`（PendingActionItems/AskPage 先行採用）；delete/push 回傳 `Promise<void>`；WS 訊息結構驗證。
- Phase 5（安全加固）：`extractURL` 協定白名單/長度上限/私有網段阻擋；兩處 ReactMarkdown `skipHtml`。
- Phase 4（拆大檔）：`MeetingDetailPage` **1040→301 行**，抽出 `components/AudioPlayer`、`TranscriptEditor`（含 SegmentRow/SpeakerRenameSheet）、`MeetingTodoForm`、`EditableMeetingTitle`、`pages/ScheduledMeetingDetail`、`hooks/useMeetingDetail`；一併完成 MeetingDetailPage 的 getIdToken→useApiCall 遷移。純結構重構、不改行為。
- **測試框架**：新增 Vitest + Testing Library（`vitest.config.ts`、`src/test/setup.ts`、`npm run test:run`），29 例覆蓋 format/url(SSRF)/client(401)/useApiCall/useFileUpload/SummarySections，全綠。
- **審查誤報排除**：字型冗餘（Vite 已按需切子集）、Markdown XSS（v10 預設不解析 raw HTML）皆不成立；P11 錯誤訊息不 genericize（後端已 curate、raw cause 只進 log）、P12 sw.ts push-debug log 保留、P14 TagEditor aria-label 早已存在。
- **待辦**：全部 Phase 已完成、typecheck/lint/build/test 全綠、尚未 merge；仍需**裝置端 e2e 回歸**（錄音上傳、檔案上傳、貼連結匯入、詳情頁各頁籤/音檔播放/逐字稿編輯/講者改名、WS 即時狀態、登入過期自動導回）後再 merge/部署。P15（MeetingList 虛擬化）延後至紀錄 >50 筆再評估。

**Phase 19–24 已 merge（`59ec949`）並部署 production（2026-07-27）**：一批六項深化單人工具的功能（RAG 跨會議問答、摘要↔音檔時間戳跳轉、貼連結匯入 YouTube/Podcast/直接音檔、情境/來源篩選+手動標籤、分享匯入、AI 自動標籤）皆已上線；CI 自動套用 migration 000014/000015、Cloud Run image 已含 yt-dlp（Dockerfile `apk add yt-dlp`）。三條功能分支合併後已刪除，本地僅剩 `main`。**剩裝置端 e2e 人工驗收（19.8 / 20.6 / 21.9 / 22.6 / 23.6 / 24.5）**；註：既有紀錄需按「重新處理」才會長出新的 AI 自動標籤與摘要時間戳（舊 prompt 產物無此欄位）。下一個開發功能待定（候選：追問式問答 / 週回顧 Digest / YouTube 播放清單批次匯入）。

**2026-07-29 修正**：iOS 錄音中裝置休眠會切斷麥克風、且結束鈕卡死——已加 Screen Wake Lock 防閒置休眠、`stop()` 對被系統中斷的 recorder 韌性收尾、新增 `interrupted` 狀態保住已錄片段（`useRecorder.ts` / `RecorderPanel.tsx`）。已部署,待真機 e2e 驗收。

Phase 19（RAG 跨會議問答 F-QA）程式完成：使用者對本人全部會議自然語言提問，語意檢索片段 → LLM 生成帶 [n] 引用的答案（單次問答、無狀態、**無新增資料表**、無相關片段不呼叫 LLM 的成本護欄）。八成重用 Phase 15 既有 embedding/檢索/LLM 基礎設施，淨新增僅 `Answerer`/`QARetriever` port + `SearchSimilarForQA` 檢索（top-K 不收斂）+ `QAUC` + `handler/qa` + 前端 AskPage/「問答」Tab。全程 TDD、後端 build/test 全綠（27 packages）、前端 gen:api/typecheck/lint/build 綠。剩裝置端 e2e 人工驗收（19.8）+ merge/部署。
Phase 17（紀錄情境化 F-SCENARIO：會議/閒聊模板 + 結構化摘要區塊）程式完成：後端 build/vet/test 全綠（24 packages）、前端 typecheck/lint/build 綠、本地 migration 000009 已套用、前後端本地已起。已再擴充第三情境「面試」（interview，17.8 ✅：migration 000010、面試 prompt、翠綠配色）。本 session 另含錄音頁情境配色化（會議黃/閒聊藍/面試綠，含大錄音鈕/光環/背景）、紀錄詳情頁改版（貼底 mini-player + hero 摘要 + meta 行 + 移除品牌列 + 完成狀態隱藏 + 頁籤 sticky）、品牌化載入動畫、確認彈窗改 Portal（修長頁被推到頁尾）。另對標競品 Aimture Shorts 擴充三項（同分支）：17.9 摘要卡片化（SummaryPoint heading/text/speaker＋講者徽章）、Phase 18 行動項到期日解析＋Web Push 到期提醒＋.ics 加入行事曆。全程 TDD、後端 build/vet/test 全綠、前端 typecheck/lint/build 綠、sqlc/openapi/TS client 重生成。剩裝置端 e2e 人工驗收（三情境各看對應區塊、卡片版型/講者徽章、dueISO 解析、Push 到期提醒、iOS 加入行事曆）與本地 migration 000011 套用 + merge/部署。
Phase 16（語者辨識 diarization：Deepgram + 講者改名 + 音檔播放）已 merge（`1da9020`）並部署 production：2026-07-22 實測 `DEEPGRAM_API_KEY` 已掛 Cloud Run（secret `busy-bee-deepgram-key:1`）、migration 000007 已隨 CI 自動套用、prod serving commit `687d52f`。全數完成（16.7 ✅）。
Phase 15（RAG 語意搜尋：pgvector + Gemini embedding）已 merge（`805659b`）並部署：migration 000006（含 `CREATE EXTENSION vector`）已隨 CI 套用、prod 已上線。僅剩 prod 裝置端 e2e 人工驗收（搜「定價」找「價格策略」）— 15.9。
Phase 14.3（錄音/上傳邊角情境）程式已具備，僅待裝置端人工驗收。
Phase 13 已 merge main（F-ACTION / F-EXPORT / 深連結 / 視覺重設計 / Tab 導覽 / 行動裝置修正，人工驗收通過）。
Phase 14（F-MANAGE：會議改名、行程編輯/刪除、排程專屬詳情頁）程式完成，待人工驗收。
✅ **iPhone 推播打通（2026-07-25 實機驗收）**：測試推播與排程會議提醒皆收到（Apple 回 `201`）。
根因是 **VAPID JWT `sub` 雙重 `mailto:` 前綴**（sender 自行加一次、webpush-go 內部又加一次 → `mailto:mailto:...`）；Chrome/Firefox 容忍但 Apple 嚴格檢查回 `403 BadJwtToken`，故僅 iOS 全數失敗。已修（傳原始 email，函式庫補前綴）+ 回歸測試。
連帶修正：`403/404/410` 一律視為訂閱失效並清除（先前 403 當可重試 → 殭屍訂閱每分鐘重送把 Apple 打到 `429` 限流）。
排程會議提醒（F-REMIND）demo 觸發採**內建計時器**（`RunReminderSweep` 每分鐘；VAPID 已設即啟用）：零外部觸發、維持 scale-to-zero，限 instance 存活時觸發（demo 足夠）；production 精準/可靠方案（Cloud Tasks / 免費外部 cron）列 ARCHITECTURE 備註。
線上 Cloud Run 已設 VAPID_PUBLIC_KEY / VAPID_SUBSCRIBER_EMAIL（env）+ VAPID_PRIVATE_KEY（Secret，已驗證與公鑰配對）。
debug 工具（保留）：`POST /api/v1/push/test`（JWT，對自己訂閱即送）+ NotificationToggle「發送測試通知」鈕 + `sw.ts` `[push-debug]` log。
