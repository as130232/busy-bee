# Session Log 封存（2026）

> 2026-10-16 自 `docs/PLAN.md`〈Session Log〉原文搬出（project-docs upgrade），保留原排列順序，內容未改。

| 日期 | 完成事項 | Commit |
|------|---------|--------|
| 2026-07-25 | 通知修正（demo / 免費 / scale-to-zero）：確認 F-REMIND 前後端已完備，決策 demo 觸發用**內建計時器**（無外部 Scheduler、零額外費用）；線上 VAPID 三變數到位；新增 debug 工具 `POST /api/v1/push/test`（`application/push/test.go` + push handler + 路由）、NotificationToggle 測試鈕、`sw.ts` push-debug log，加速 iPhone 實機顯示驗證。後端 vet/test 綠、前端 build 綠 | `main` |
| 2026-07-17 | Phase 1 全部完成（1.1–1.6：骨架、apperr、config、server、response、compose）；TDD 全程；分支 feat/phase-1-backend-skeleton | `8cec0df..9fd11b5` |
| 2026-07-17 | Phase 2 全部完成（2.1–2.6：migrations、sqlc、WithTx、auth 白名單、/users/sync、openapi）；分支 feat/phase-2-db-auth | `0d560c1..ee6a642` |
| 2026-07-17 | Phase 3 全部完成（3.1–3.4：scaffold、Google 登入、API client、Dashboard）；Firebase 專案 busy-bee-502710 建立；登入人工驗收通過 | `41de695..f9d77f4` |
| 2026-07-17 | Phase 4.1 Dockerfile 完成；4.2-4.4 暫緩（Supabase 額度滿，production DB 待定）；決策：優先開發 M1-B 核心功能 | `d1ed1ce` |
| 2026-07-18 | Phase 5 全部完成（5.1–5.6：狀態機、GCS impersonation 簽名、三段式上傳 API、上傳 UI）；上傳人工驗收通過 | `324273c..a0876cf` |
| 2026-07-18 | Phase 6 全部完成（6.1–6.6：Asynq、ProcessUC、Groq STT、ffmpeg、冪等、retry）；修復完成任務擋重排 bug；e2e 繁中逐字稿驗證通過 | `06c9930..e610142` |
| 2026-07-18 | Phase 6R 完成：ADR-010 移除 Redis（記憶體佇列 + Sweeper）；無 Redis e2e 復原驗證通過 | `b64bcb2` |
| 2026-07-18 | Phase 7 完成（hub + 首訊驗證 + 事件發布 + 前端即時更新）；人工驗收通過；決定開發順序調整為 9 → 10 → 8 | `275ea01..60e25d8` |
| 2026-07-18 | Phase 9 完成（artifacts、Gemini client、雙模板、冪等生成、查詢 API）；模型名修正為 gemini-3-flash-preview；真實生成 e2e 通過 | `369234b..cfd2833` |
| 2026-07-19 | Phase 10 完成（list/search/detail/retry API + UI、逐字稿去重）；M2-A 達成；人工驗收通過 | `5eeb122..2f8a98d` |
| 2026-07-19 | Phase 8 完成（useRecorder + RecorderPanel）；錄音→STT→文件全鏈路人工驗收通過；M1-B 達成 | `cc151fc` |
| 2026-07-19 | Phase 4 部署完成（Neon + Secret Manager + Cloud Run + Hosting）；production 上線 | `f4fb15d` |
| 2026-07-19 | ADR-004 修訂 scale-to-zero（月費 ~$12→~$0-2）；4.4 CI/CD 完成（WIF），首次執行全綠自動部署 revision 00004；Phase 4 全數完成 | `75ec206..88742a1` |
| 2026-07-19 | Phase 12 完成（rate limiting、錯誤畫面、README） | `383adc6..88426d4` |
| 2026-07-19 | Phase 11 程式完成（訂閱/VAPID/掃描發送/SW/排程 UI）；後端發送實測成功；文件稽核：清除 ARCHITECTURE 5 處過時「計畫中」標籤、補 PRODUCT 驗收勾選 | `bb48b3a..066f846` |
| 2026-07-19 | 後端健檢修正（error_message 不外洩原文、rate limit 惰性清理）；Phase 11 merge main；Cloud Run 補 VAPID env/secret（rev 00007）；CI 自動部署 | `8cc6f78` |
| 2026-07-19 | 前端視覺重設計：Linear 式暗色風、Tailwind v4 + design tokens、暗/亮雙主題、行動優先（iPhone PWA 為主）、lucide 圖示、大圓形錄音鈕、排程 bottom sheet；typecheck/lint/build 通過，待 iPhone 實機驗收 | `90c97a3` |
| 2026-07-19 | Phase 13 擴充第一波：F-ACTION（LLM 抽取行動項→artifacts JSON 冪等標記→action_items 表→list/pending/toggle API→Dashboard 待辦卡＋詳情 tab）、F-EXPORT（複製/下載/Web Share）、F-REMIND 深連結（`/?record=1`＋SW 聚焦既有分頁＋錄音鈕高亮）；後端全測試綠、路由實測 401、前端三檢通過；待真實音訊 e2e 人工驗收 | `1e9c741..9cd5595` |
| 2026-07-19 | 前端全站 CSS 動畫（進場淡入、列表 stagger、錄音 ripple、sheet 滑入、勾選彈跳；尊重 prefers-reduced-motion）；Phase 13 分支 merge main | `8361753..91b4935` |
| 2026-07-19 | 前端改底部 Tab 導覽（分支 feat/tab-navigation）：拆單頁為錄音/會議/行程/設定四分頁 + TabBar/TabLayout/TopBar/MeetingList/useMeetings；詳情頁維持獨立；三檢通過、路由導向驗證 | `b8bb2b3` |
| 2026-07-19 | 行動裝置修正：PWA 禁縮放、錄音頁三段固定版面一屏不滑、排程 sheet 不被遮、通知開關防卡死 | `3a2a37f` |
| 2026-07-20 | Phase 13 人工驗收通過（除深連結，卡推播顯示）；Phase 14 程式完成：改名/刪除 API（TDD）+ 排程專屬詳情頁 + ScheduleSheet 編輯模式 + 行程頁過濾上傳暫存；盤點確認錄音/上傳邊角程式已存在僅待驗收 | `3f6d23b, 9a4b756` |
| 2026-07-19 | 找到提醒不觸發根因（scale-to-zero 下 instance=0，進程內 sweeper 不跑）：新增密鑰保護的 `/internal/sweep-reminders` 端點備用（TDD 3 例）。**決策：暫緩自動提醒**——每分鐘 Scheduler 會讓 instance 常駐、失去 scale-to-zero 省錢意義；使用者選擇先不啟用，端點休眠（無密鑰無觸發器＝零額外費用）。未來若要啟用，較省的方向是 Cloud Tasks 精準排程（提醒時刻才喚醒一次） | `49bb919` |
| 2026-07-20 | Phase 15 RAG 語意搜尋程式完成（15.1–15.8）：pgvector 前置＋migration 000006（transcript_chunks、hnsw cosine、CASCADE）→ domain/search entity/ports＋SplitIntoChunks 切塊（TDD）→ Gemini Embed（gemini-embedding-001 @768 維）→ chunk_repo（手寫 pgx＋pgvector-go，DISTINCT ON＋`<=>`＋owner 過濾，真實 PG 整合測試）→ IndexUC/SearchUC（hybrid 合併＋embedding 失敗降級純字面，TDD）→ worker 回填掃描＋completed best-effort 觸發 → List handler 走 SearchUC 回 matchSnippet＋openapi/TS client＋main.go wiring＋前端片段顯示。全後端 build/vet/test 綠、前端 typecheck/build 綠；待本地 e2e 人工驗收（搜「定價」找「價格策略」）＋Neon extension＋merge（15.9） | `c6a0726..3e1f969` |
| 2026-07-21 | Phase 16 語者辨識完成（16.1–16.6）＋本地 e2e 驗收通過：domain segments/FlattenSegments（TDD）→ migration 000007（transcript_segments/speaker_names jsonb）＋repo round-trip → STT 換 **Deepgram nova-2 zh-TW**（聲學分離；踩坑：nova-3+multi 中文空、Gemini 音訊誤拆多人皆棄）→ `UpdateSpeakerNames` 改名 use case＋STT 空結果保護 → `PATCH /speakers`＋音檔播放端點 `GET /audio-url`（GCS SignedDownloadURL）→ 前端分講者顯示（晶片配色/時間碼靠左可點跳播）＋改名 sheet＋音檔播放器＋匯出用顯示名＋列表時長分秒 tag＋返回回列表＋標題跑馬燈。全後端 build/vet/test 綠、前端 typecheck/lint/build 綠；清理拋棄式 POC。待 merge＋部署（Deepgram key 進 Secret Manager、prod migration 000007）(16.7) | `f53fb6c..1da9020` |
| 2026-07-22 | 部署收尾實測驗證：Phase 15/16 皆已 merge 並上線。gcloud 確認 `DEEPGRAM_API_KEY` 已掛 Cloud Run（secret `busy-bee-deepgram-key:1`，runtime SA busy-bee-storage）＋Groq/Gemini/VAPID 皆在；prod `/version` 回 commit `687d52f`（main HEAD，晚於 15/16 merge），revision `00023-w9m` serving，故 CI 自動 migration 000006（含 `CREATE EXTENSION vector`）/000007 已套用。16.7 標 ✅、Phase 16 overview ✅；15.9 部署部分完成、僅剩 prod 裝置端 e2e 驗收。**結論：部署/基建零待辦，剩裝置端人工驗收（15.9 RAG、14.3 邊角、Phase 16 diarization e2e）** | — |
| 2026-07-22 | Phase 17 紀錄情境化程式完成（17.1–17.6）：migration 000009（scenario/summary_sections）→ domain Scenario/ParseScenario/SummarySection/Summarizer port → 兩情境 prompt + gemini Summarize → 管線改產結構化區塊（冪等）並移除 PRD/TechSpec 自動產生（LLM 3→2 呼叫）→ openapi/handler 串接 → 前端「會議」頁籤改「紀錄」、錄音前情境分段切換、清單情境標籤（移到標題前、時長改純文字區隔）、SummarySections 通用渲染器、詳情頁整合、錄音標題依情境命名。後端 build/vet/test 綠（24 pkg，含改寫 process 測試 + ParseScenario 測試）、前端 typecheck/lint/build 綠。待裝置端 e2e + merge（17.7） | `feat/scenario-templates` |
| 2026-07-22 | 對標競品 Aimture Shorts 擴充三項：**17.9** 摘要卡片化（SummaryPoint heading/text/speaker＋容忍式 unmarshal、prompt 分情境密度、SummarySections 單一渲染器＋講者徽章、共用 speakerColor）；**Phase 18** 行動項到期日（dueISO 解析＋migration 000011 due_at/reminded_at＋ReminderUC 掃描到期行動項推「待辦提醒」複用既有 push/sweeper）＋**.ics 行事曆**（純前端 services/ics.ts、ActionItemList「加入行事曆」鈕）。全程 TDD（SummaryPoint 3 例、DueDate 4 例、行動項提醒 2 例）；後端 build/vet/test 全綠、前端 typecheck/lint/build 綠、sqlc/openapi/TS client 重生成、本地 migration 000011 待套用。待裝置端 e2e + merge | `feat/scenario-templates` |
| 2026-07-22 | 詳情頁 UX + 手動待辦（18.7–18.8，實測回饋驅動）：摘要改頁籤（預設選中）＋頁籤移標題下 sticky＋命名統一「待辦」；手動新增待辦全端（migration 000012 source 欄位、重跑只刪 llm 保護手動項、AddUC owner 驗證 TDD、POST 端點、前端 AddTodoForm）。後端 build/vet/test 全綠、前端三檢綠、本地 migration 000012 已套用、後端已重啟含新路由 | `feat/scenario-templates` |
| 2026-07-22 | UI 調整 + 情境擴充「面試」（17.8）：① 品牌化載入動畫 `Loader`（蜂巢 sonar/breathe）；② 確認彈窗抽出 `Sheet` 改 React Portal 掛 body，修正 AppShell transform 讓 fixed 被推到頁尾（iOS）；③ 紀錄詳情頁改版：貼底 mini-player（AudioPlayer portal）、hero 摘要合併雙卡、meta 行（情境·時長·講者數·行動項）、頁籤 sticky、移除全域品牌列（`AppShell hideTopBar`）、完成狀態不再顯示（詳情＋列表）；④ 錄音頁情境配色化 `scenarioTheme`（會議黃/閒聊藍/面試綠，含大鈕/聲波環/光點/背景暈染）、情境切換上移放大；⑤ 面試情境全端：migration 000010（CHECK 放寬，已套用）、domain 常數/IsValid、`summary_interview.md`+gemini 註冊、openapi enum、重生 TS client、前端 label/theme/toggle/清單標籤。後端 build/test 綠、前端 typecheck/lint 綠。待裝置端 e2e | `feat/scenario-templates` |
