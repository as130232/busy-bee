# Busy Bee 開發計畫與進度追蹤

> 依 `docs/PRODUCT.md` 的 F-ID 與優先序切分 Phase，記錄任務狀態與進度。
> 更新日期：2026-07-19

---

## 當前焦點

Phase 11 已 merge main 並部署 production（Cloud Run VAPID 設定完成，revision 00007+）。
剩最後一步：用戶驗收通知顯示（macOS Chrome）→ 勾 PRODUCT F-REMIND 剩餘驗收 → MVP 完成。

---

## 狀態說明

| 符號 | 含義 |
|------|------|
| ✅ | 已完成 |
| 🔄 | 進行中 |
| ⬜ | 待實作 |
| ⏸ | 暫緩（等待前置依賴） |
| 🔁 | 重做中（原已 ✅ 但發現問題需要修復） |
| ❌ | 取消（保留紀錄，永不刪除） |

---

## 進度總覽

| 狀態 | 階段 | 里程碑 |
|------|------|--------|
| ✅ | Phase 1 — 後端骨架與基建 | M1-A |
| ✅ | Phase 2 — DB 與 Auth | M1-A |
| ✅ | Phase 3 — 前端骨架與登入 | M1-A |
| ✅ | Phase 4 — 部署管線 | M1-A |
| ✅ | Phase 5 — 上傳流程 | M1-B |
| ✅ | Phase 6 — 任務佇列與 STT | M1-B |
| ✅ | Phase 6R — 佇列簡化（移除 Redis） | M1-B |
| ✅ | Phase 7 — WebSocket 通知 | M1-B |
| ✅ | Phase 8 — 錄音 UI | M1-B |
| ✅ | Phase 9 — LLM 文件生成 | M2-A |
| ✅ | Phase 10 — 歷史與搜尋 | M2-A |
| 🔄 | Phase 11 — 提醒與推播 | M2-B |
| ✅ | Phase 12 — Production 完善 | M2-B |

---

## Phase 1：後端骨架與基建
> 里程碑：M1-A | ✅ 完成於 2026-07-17
> 建立 Go 專案骨架與橫向基建（apperr / config / response），對齊 sport-hub 模式。

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ✅ | 1.1 | Monorepo 目錄與 go.mod | `busy-bee-be/go.mod` | Go 1.26；busy-bee-fe/ 佔位；已加入 project go.work | `8cec0df` |
| ✅ | 1.2 | apperr 與 errcode 基建 | `busy-bee-be/pkg/apperr/`, `busy-bee-be/pkg/consts/errcode/` | Code + Params + Cause；ClientMsg 不含 cause | `c0eb117` |
| ✅ | 1.3 | env config 載入 | `busy-bee-be/infrastructure/config/` | OS env > .env.{APP_ENV} > 預設值 | `a83ebde` |
| ✅ | 1.4 | Gin server + health check | `busy-bee-be/cmd/server/main.go`, `busy-bee-be/interface/http/server.go` | Recovery→RequestID→Logger；SIGTERM graceful shutdown 已實測 | `89cc7a9` |
| ✅ | 1.5 | response envelope | `busy-bee-be/interface/http/response/` | OK / Fail；unknown error 不外洩 cause | `0c0a732` |
| ✅ | 1.6 | Docker Compose 本地環境 | `docker-compose.yml` | PG16 + Redis7 healthcheck 通過；含 .env.example | `9fd11b5` |

---

## Phase 2：DB 與 Auth（F-AUTH）
> 里程碑：M1-A | ✅ 完成於 2026-07-17
> 資料庫接入與 Firebase 身份驗證，完成用戶同步。

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ✅ | 2.1 | migrations 骨架 + users 表 | `busy-bee-be/db/migrations/` | embedded + cmd/migrate；up/down 實測可逆 | `0d560c1` |
| ✅ | 2.2 | sqlc 設定 + users query | `busy-bee-be/db/sqlc.yaml`, `busy-bee-be/db/query/users.sql` | upsert by firebase_uid；產出 sqlcgen | `fa9237f` |
| ✅ | 2.3 | pgx pool + WithTx helper | `busy-bee-be/infrastructure/db/` | commit/rollback/panic 皆有整合測試 | `d4978ef` |
| ✅ | 2.4 | Firebase auth middleware | `busy-bee-be/interface/http/middleware/auth.go` | TokenVerifier port；白名單 fail-closed、大小寫不敏感 | `460edd7` |
| ✅ | 2.5 | POST /users/sync | `busy-bee-be/interface/http/handler/user/` | Deps 注入組裝；200/401 煙霧測試通過 | `4a0c408` |
| ✅ | 2.6 | openapi.yaml 初版 | `busy-bee-be/api/openapi.yaml` | health + users/sync + Envelope schema | `ee6a642` |

---

## Phase 3：前端骨架與登入（F-AUTH）
> 里程碑：M1-A | ✅ 完成於 2026-07-17

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ✅ | 3.1 | Vite React PWA scaffold | `busy-bee-fe/` | vite-plugin-pwa manifest + SW；dev proxy 到後端 | `41de695` |
| ✅ | 3.2 | Google 登入流程 | `busy-bee-fe/src/hooks/useAuth.tsx`, `busy-bee-fe/src/services/firebase.ts` | signInWithPopup + 後端 sync；403 登出並提示；實測通過 | `f9d77f4` |
| ✅ | 3.3 | API client 生成 | `busy-bee-fe/src/services/api/` | openapi-typescript 型別 + envelope 解析 ApiError | `82a3d0d` |
| ✅ | 3.4 | Dashboard shell + auth guard | `busy-bee-fe/src/pages/`, `busy-bee-fe/src/components/RequireAuth.tsx` | /login 公開、/ 受保護；session 恢復重同步 | `f9d77f4` |

---

## Phase 4：部署管線
> 里程碑：M1-A | ✅ 完成於 2026-07-19
> production DB 採 Neon 免費方案（Singapore）。上線網址 https://busy-bee-502710.web.app；API https://busy-bee-api-897794325314.asia-east1.run.app。
> scale-to-zero（ADR-004 修訂）；main push = 自動測試 + 部署（WIF 零金鑰）。

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ✅ | 4.1 | Dockerfile | `busy-bee-be/Dockerfile` | multi-stage、非 root、含 ffmpeg；交叉編譯避開 QEMU panic | `d1ed1ce, f4fb15d` |
| ✅ | 4.2 | Cloud Run 部署 + Secret Manager | — | Neon migration v3；三 secrets；min-instances=1 + no-cpu-throttling；runtime SA = busy-bee-storage（self signBlob） | — |
| ✅ | 4.3 | Firebase Hosting 部署 | `busy-bee-fe/firebase.json` | /api rewrite → Cloud Run；WS 直連（VITE_WS_BASE）；bucket CORS 加 hosting domains | `f4fb15d` |
| ✅ | 4.4 | GitHub Actions CI/CD | `.github/workflows/deploy.yml` | WIF 零金鑰；PR 跑測試（真實 PG + ffmpeg）；main 自動 migration + 雙部署；首次執行全綠 | `88742a1` |

---

## Phase 5：上傳流程（F-UPLOAD）
> 里程碑：M1-B | ✅ 完成於 2026-07-18
> GCS signed URL 三段式直傳（詳見 ARCHITECTURE.md ADR-001）。

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ✅ | 5.1 | meetings 表 migration + query | `busy-bee-be/db/migrations/`, `busy-bee-be/db/query/meetings.sql` | 狀態機 + 樂觀鎖 UpdateStatus + owner-only | `324273c` |
| ✅ | 5.2 | GCS infra | `busy-bee-be/infrastructure/gcs/` | IAM impersonation 簽名（零金鑰檔）；真實上傳整合測試 | `cb5496b` |
| ✅ | 5.3 | POST /meetings | `busy-bee-be/application/meeting/create.go` | 建記錄（scheduled）+ signed URL；content-type 白名單 | `30197a3` |
| ✅ | 5.4 | POST /meetings/{id}/complete-upload | `busy-bee-be/application/meeting/complete_upload.go` | 驗物件存在 → pending；冪等；ResolveUser middleware | `30197a3` |
| ✅ | 5.5 | GCS bucket CORS 設定 | — | bucket busy-bee-502710-audio（asia-east1）+ CORS，併入 5.2 | `cb5496b` |
| ✅ | 5.6 | 拖曳/選檔上傳 UI | `busy-bee-fe/src/components/UploadZone.tsx` | XHR 進度 + 失敗重試；上傳實測通過 | `a0876cf` |

---

## Phase 6：任務佇列與 STT（F-PIPELINE）
> 里程碑：M1-B | ✅ 完成於 2026-07-18
> e2e 驗證：真實上傳音訊 → Groq STT → completed，繁中逐字稿入庫。

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ✅ | 6.1 | Asynq client/server 接入 | `busy-bee-be/infrastructure/queue/asynq.go` | TaskID 去重；已結束任務可重排（Inspector）；測試用獨立 Redis DB | `06c9930, e610142` |
| ✅ | 6.2 | process_meeting worker | `busy-bee-be/worker/process_meeting.go`, `busy-bee-be/application/meeting/process.go` | 分階段冪等狀態機；最後一次重試失敗才標 failed | `7368987` |
| ✅ | 6.3 | Groq Whisper client | `busy-bee-be/infrastructure/stt/client.go` | 串流 multipart；繁中 prompt 引導 | `b8608e7, e610142` |
| ✅ | 6.4 | ffmpeg 壓縮 fallback | `busy-bee-be/infrastructure/stt/compress.go` | 超限轉 16kbps mono mp3；真實 ffmpeg 測試 | `b8608e7` |
| ✅ | 6.5 | 冪等處理 | `busy-bee-be/application/meeting/process.go` | transcript 已存在跳過 STT（不重複扣費） | `7368987` |
| ✅ | 6.6 | retry 上限與 failed 處理 | `busy-bee-be/worker/process_meeting.go` | MaxRetry 3；error_message 入庫；cmd/enqueue 手動重排 | `b8608e7` |

---

## Phase 6R：佇列簡化（移除 Redis）
> 里程碑：M1-B | ✅ 完成於 2026-07-18
> ADR-010：記憶體佇列 + DB 復原掃描取代 Asynq/Redis；鎖定單 instance。e2e 驗證：關閉 Redis 容器後 Sweeper 自動復原 pending 會議完成 STT。

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ✅ | 6R.1 | 記憶體佇列 | `busy-bee-be/infrastructure/queue/memory.go` | in-flight 去重、backoff retry、背壓、graceful drain；含 -race 測試 | — |
| ✅ | 6R.2 | Sweeper 復原掃描 | `busy-bee-be/worker/sweeper.go` | 啟動 + 每 2 分鐘掃未完成會議重新入列 | — |
| ✅ | 6R.3 | 清除 Redis 依賴 | `docker-compose.yml`, `busy-bee-be/cmd/server/main.go` | 移除 asynq/redis/cmd-enqueue；config 移除 RedisConfig | — |

## Phase 7：WebSocket 通知（F-STATUS）
> 里程碑：M1-B | ✅ 完成於 2026-07-18
> ADR-002 修訂版：單機 in-process hub（ADR-010）。上傳後狀態即時流動實測通過。

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ✅ | 7.1 | WS hub 連線管理 | `busy-bee-be/interface/http/ws/hub.go` | 多連線/用戶、慢連線丟訊息；race 測試 | `275ea01` |
| ✅ | 7.2 | WS 第一則訊息 JWT 驗證 | 同上 | 10s 逾時；白名單；驗證前零推送 | `275ea01` |
| ❌ | 7.3 | Redis Pub/Sub fan-out | — | 取消：ADR-010 單 instance，改 in-process notifier | — |
| ✅ | 7.4 | worker 發布狀態事件 | `busy-bee-be/application/meeting/process.go` | StatusNotifier port；含 failed 事件 | `275ea01` |
| ✅ | 7.5 | useWebSocket hook | `busy-bee-fe/src/hooks/useMeetingStatusSocket.ts` | 開連線帶新鮮 token；退避重連 | `60e25d8` |
| ✅ | 7.6 | Dashboard 即時狀態顯示 | `busy-bee-fe/src/pages/DashboardPage.tsx` | 事件即地更新列表 | `60e25d8` |

---

## Phase 8：錄音 UI（F-RECORD）

> 里程碑：M1-B
> M1-B 驗收：錄音/上傳後即時看到 transcribing → completed。

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ✅ | 8.1 | useRecorder hook | `busy-bee-fe/src/hooks/useRecorder.ts` | mime 自動偵測；1s chunk；停止釋放麥克風 | `cc151fc` |
| ✅ | 8.2 | 錄音 UI | `busy-bee-fe/src/components/RecorderPanel.tsx` | 開始/暫停/繼續/捨棄；beforeunload 警告 | `cc151fc` |
| ✅ | 8.3 | 錄音接上傳流程 | 同上 | 結束即走直傳；失敗可重試 | `cc151fc` |
| ✅ | 8.4 | 瀏覽器相容性處理 | `busy-bee-fe/src/hooks/useRecorder.ts` | Safari mp4 fallback；權限拒絕明確指引 | `cc151fc` |

---

## Phase 9：LLM 文件生成（F-DOCGEN）
> 里程碑：M2-A | ✅ 完成於 2026-07-18
> e2e：真實 Gemini（gemini-3-flash-preview）生成 PRD + Tech Spec；防幻覺規則生效（未討論章節標註「會議未討論」）。

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ✅ | 9.1 | artifacts 表 migration + query | `busy-bee-be/db/migrations/`, `busy-bee-be/db/query/artifacts.sql` | UNIQUE(meeting,type) 冪等 upsert | `369234b` |
| ✅ | 9.2 | Gemini client | `busy-bee-be/infrastructure/llm/gemini.go` | genai SDK；模型 env 可切換（預設 gemini-flash-latest） | `cfd2833` |
| ✅ | 9.3 | PRD prompt template | `busy-bee-be/infrastructure/llm/prompts/prd.md` | Q2 七章骨架 + 防幻覺鐵則；embedded | `cfd2833` |
| ✅ | 9.4 | Tech Spec prompt template | `busy-bee-be/infrastructure/llm/prompts/tech_spec.md` | 同上 | `cfd2833` |
| ✅ | 9.5 | worker 整合生成階段 | `busy-bee-be/application/meeting/process.go` | 逐文件冪等（retry 只補缺的） | `cfd2833` |
| ✅ | 9.6 | artifacts 查詢 API | `busy-bee-be/interface/http/handler/meeting/` | GET /meetings/{id}/artifacts；owner-only | `cfd2833` |

---

## Phase 10：歷史與搜尋（F-HISTORY、F-SEARCH）

> 里程碑：M2-A
> M2-A 驗收：完整會議 → 文件流程可用、可搜尋（MVP 上線）。

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ✅ | 10.1 | meetings list/detail API | `busy-bee-be/application/meeting/list.go` | owner-only；列表不含 transcript | `5eeb122` |
| ✅ | 10.2 | ILIKE 搜尋 API | `busy-bee-be/db/query/meetings.sql` | title + transcript；跨用戶隔離整合測試 | `5eeb122` |
| ✅ | 10.3 | 列表/詳情 UI + Markdown 渲染 | `busy-bee-fe/src/pages/MeetingDetailPage.tsx` | PRD/TechSpec/逐字稿三頁籤；WS 事件自動刷新 | `16fcd89` |
| ✅ | 10.4 | 搜尋 UI | `busy-bee-fe/src/pages/DashboardPage.tsx` | 防抖 300ms | `16fcd89` |
| ✅ | 10.5 | 失敗會議顯示 + 手動 retry | 前後端 | failed→pending 樂觀鎖 + 重新入列 | `5eeb122, 16fcd89` |
| ✅ | 10.6 | 逐字稿去重修正 | `busy-bee-be/infrastructure/stt/client.go` | 分段組稿去除 Whisper 相鄰重複幻覺；實測驗證 | `2f8a98d` |

---

## Phase 11：提醒與推播（F-REMIND）
> 里程碑：M2-B | 🔄 程式完成，人工驗收中
> 後端發送已實測成功（reminder.sent delivered=1，掃描/防重複/1 分鐘提前皆正確）；通知「顯示」待用戶端環境排查（macOS 通知設定）。

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ✅ | 11.1 | push_subscriptions 表 + 訂閱 API | `busy-bee-be/db/migrations/000004_*`, `busy-bee-be/interface/http/handler/push/` | reminded_at 防重複 + 部分索引 | `bb48b3a` |
| ✅ | 11.2 | VAPID key + web-push 發送 | `busy-bee-be/infrastructure/webpush/` | 私鑰 Secret Manager；410 Gone 清訂閱 | `bb48b3a` |
| ✅ | 11.3 | SW push handler + 前端訂閱 | `busy-bee-fe/src/sw.ts`, `busy-bee-fe/src/components/NotificationToggle.tsx` | injectManifest 自訂 SW；iOS 加入主畫面提示 | `47de677` |
| ✅ | 11.4 | scheduled meeting CRUD | `busy-bee-be/application/meeting/schedule.go` | 建立/編輯；編輯清 reminded_at；TDD | `bb48b3a` |
| ✅ | 11.5 | 提醒排程（掃描式） | `busy-bee-be/application/meeting/reminder.go`, `busy-bee-be/worker/reminder.go` | 每分鐘掃；暫時失敗重試、Gone 清除；TDD 4 例 | `bb48b3a` |

---

## Phase 12：Production 完善
> 里程碑：M2-B | ✅ 完成於 2026-07-19

> 里程碑：M2-B

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ✅ | 12.1 | rate limiting middleware | `busy-bee-be/interface/http/middleware/ratelimit.go` | 每 IP 10rps/burst30；閒置清理；429 envelope | `383adc6` |
| ✅ | 12.2 | error state UI 梳理 | `busy-bee-fe/src/` | 列表載入失敗提示 + 重載；常見錯誤碼友善訊息 | `38a229d` |
| ✅ | 12.3 | Cloud Run 參數調整 | — | 併入 Phase 4：scale-to-zero + no-cpu-throttling（ADR-004 修訂） | — |
| ✅ | 12.4 | README + .env.example 收尾 | `README.md`, `.env.example` | 架構亮點 + ADR 導覽 + 本地開發指南 | — |

---

## 依賴關係圖

```text
Phase 1（後端骨架）
  └─ Phase 2（DB 與 Auth）
       ├─ Phase 3（前端登入）
       │    └─ Phase 4（部署管線）
       │         └─ Phase 11（提醒與推播）
       └─ Phase 5（上傳流程）
            ├─ Phase 6（佇列與 STT）
            │    ├─ Phase 7（WS 通知）
            │    └─ Phase 9（LLM 生成）
            │         └─ Phase 10（歷史與搜尋）
            │              └─ Phase 12（Production 完善）
            └─ Phase 8（錄音 UI）

Phase 7 / 8 / 9 完成 Phase 6 後可平行進行
```

---

## Session Log

| 日期 | 完成事項 | Commit |
|------|---------|--------|
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

---

## 驗證方式（每個 Phase 完成後）

後端（busy-bee-be/）：

1. `go build ./...` — 無編譯錯誤
2. `go vet ./...` — 無警告
3. `go test ./...` — 所有測試通過
4. `docker compose up -d` + 啟動 server + `curl :8080/health` — 服務正常啟動

前端（busy-bee-fe/）：

1. `npm run typecheck` — 無 TypeScript 錯誤
2. `npm run lint` — 無新增 lint 警告
3. `npm run build` — production build 成功
