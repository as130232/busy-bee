# Busy Bee 開發計畫與進度追蹤

> 依 `docs/PRODUCT.md` 的 F-ID 與優先序切分 Phase，記錄任務狀態與進度。
> 更新日期：2026-07-17

---

## 當前焦點

Phase 2 已完成（DB + Firebase Auth + /users/sync + OpenAPI 初版）。
下一步：實作 Phase 3.1 Vite React PWA scaffold；動工前需先建立 Firebase 專案（console 手動步驟）。

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
| ⬜ | Phase 3 — 前端骨架與登入 | M1-A |
| ⬜ | Phase 4 — 部署管線 | M1-A |
| ⏸ | Phase 5 — 上傳流程 | M1-B |
| ⏸ | Phase 6 — 任務佇列與 STT | M1-B |
| ⏸ | Phase 7 — WebSocket 通知 | M1-B |
| ⏸ | Phase 8 — 錄音 UI | M1-B |
| ⏸ | Phase 9 — LLM 文件生成 | M2-A |
| ⏸ | Phase 10 — 歷史與搜尋 | M2-A |
| ⏸ | Phase 11 — 提醒與推播 | M2-B |
| ⏸ | Phase 12 — Production 完善 | M2-B |

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

> 里程碑：M1-A

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ⬜ | 3.1 | Vite React PWA scaffold | `busy-bee-fe/` | manifest + service worker 骨架 | — |
| ⬜ | 3.2 | Google 登入流程 | `busy-bee-fe/src/pages/`, `busy-bee-fe/src/services/` | Firebase SDK；登入後呼叫 /users/sync | — |
| ⬜ | 3.3 | API client 生成 | `busy-bee-fe/src/services/api/` | 由 openapi.yaml 生成 TS client | — |
| ⬜ | 3.4 | Dashboard shell + auth guard | `busy-bee-fe/src/pages/` | 路由、未登入導向登入頁 | — |

---

## Phase 4：部署管線

> 里程碑：M1-A
> M1-A 驗收：可登入看到空 Dashboard、CI/CD 跑通。

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ⬜ | 4.1 | Dockerfile | `busy-bee-be/Dockerfile` | multi-stage；含 ffmpeg | — |
| ⬜ | 4.2 | Cloud Run 部署 + Secret Manager | — | min-instances=1、CPU always allocated | — |
| ⬜ | 4.3 | Firebase Hosting 部署 | `busy-bee-fe/firebase.json` | | — |
| ⬜ | 4.4 | GitHub Actions CI/CD | `.github/workflows/deploy.yml` | test → build → deploy | — |

---

## Phase 5：上傳流程（F-UPLOAD）

> 里程碑：M1-B
> GCS signed URL 三段式直傳（詳見 ARCHITECTURE.md ADR-001）。

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ⏸ | 5.1 | meetings 表 migration + query | `busy-bee-be/db/migrations/`, `busy-bee-be/db/query/meetings.sql` | 狀態機欄位；待 Phase 2 完成 | — |
| ⏸ | 5.2 | GCS infra | `busy-bee-be/infrastructure/gcs/` | signed URL 產生（大小/類型限制）、下載 | — |
| ⏸ | 5.3 | POST /meetings | `busy-bee-be/interface/http/handler/meeting/`, `busy-bee-be/application/meeting/create.go` | 建記錄 + 回 signed URL | — |
| ⏸ | 5.4 | POST /meetings/{id}/complete-upload | 同上 | 驗證物件存在 → 狀態 pending | — |
| ⏸ | 5.5 | GCS bucket CORS 設定 | — | 允許 FE domain；lifecycle 規則 | — |
| ⏸ | 5.6 | 拖曳/選檔上傳 UI | `busy-bee-fe/src/components/` | 直傳 GCS + 進度顯示 + 失敗重試 | — |

---

## Phase 6：任務佇列與 STT（F-PIPELINE）

> 里程碑：M1-B

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ⏸ | 6.1 | Asynq client/server 接入 | `busy-bee-be/infrastructure/queue/` | 與 HTTP 同 binary；graceful shutdown；待 Phase 5 | — |
| ⏸ | 6.2 | process_meeting worker | `busy-bee-be/worker/process_meeting.go`, `busy-bee-be/application/meeting/process.go` | 狀態機推進 | — |
| ⏸ | 6.3 | Groq Whisper client | `busy-bee-be/infrastructure/stt/` | 實作 domain STTClient interface | — |
| ⏸ | 6.4 | ffmpeg 壓縮 fallback | `busy-bee-be/infrastructure/stt/` | 超過大小上限時轉 16kbps mono mp3 | — |
| ⏸ | 6.5 | 冪等處理 | `busy-bee-be/application/meeting/process.go` | 每階段先查 status，已完成則跳過 | — |
| ⏸ | 6.6 | retry 上限與 failed 處理 | `busy-bee-be/worker/` | 記錄 error_message | — |

---

## Phase 7：WebSocket 通知（F-STATUS）

> 里程碑：M1-B
> 跨 instance fan-out 設計詳見 ARCHITECTURE.md ADR-002。

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ⏸ | 7.1 | WS hub 連線管理 | `busy-bee-be/interface/http/handler/ws.go` | goroutine 生命週期（人工必審）；待 Phase 6 | — |
| ⏸ | 7.2 | WS 第一則訊息 JWT 驗證 | 同上 | 驗證前不綁定 user、不推送 | — |
| ⏸ | 7.3 | Redis Pub/Sub fan-out | `busy-bee-be/infrastructure/queue/` | 各 instance 訂閱後轉發 | — |
| ⏸ | 7.4 | worker 發布狀態事件 | `busy-bee-be/application/meeting/process.go` | 每次狀態變更 publish | — |
| ⏸ | 7.5 | useWebSocket hook | `busy-bee-fe/src/hooks/useWebSocket.ts` | 自動重連 + 重連後拉最新狀態 | — |
| ⏸ | 7.6 | Dashboard 即時狀態顯示 | `busy-bee-fe/src/components/` | | — |

---

## Phase 8：錄音 UI（F-RECORD）

> 里程碑：M1-B
> M1-B 驗收：錄音/上傳後即時看到 transcribing → completed。

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ⏸ | 8.1 | useRecorder hook | `busy-bee-fe/src/hooks/useRecorder.ts` | MediaRecorder；不寫死 mime type；待 Phase 5 | — |
| ⏸ | 8.2 | 錄音 UI | `busy-bee-fe/src/components/` | 開始/暫停/結束、離開警告 | — |
| ⏸ | 8.3 | 錄音接上傳流程 | `busy-bee-fe/src/` | 結束後走 F-UPLOAD 直傳 | — |
| ⏸ | 8.4 | 瀏覽器相容性處理 | `busy-bee-fe/src/hooks/` | Safari mp4/aac；不支援時明確錯誤 | — |

---

## Phase 9：LLM 文件生成（F-DOCGEN）

> 里程碑：M2-A

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ⏸ | 9.1 | artifacts 表 migration + query | `busy-bee-be/db/migrations/`, `busy-bee-be/db/query/artifacts.sql` | 待 Phase 6 完成 | — |
| ⏸ | 9.2 | Gemini client | `busy-bee-be/infrastructure/llm/` | 實作 domain LLMClient interface | — |
| ⏸ | 9.3 | PRD prompt template | `busy-bee-be/infrastructure/llm/prompts/` | 章節模板依 PRODUCT.md Q2（人工必審） | — |
| ⏸ | 9.4 | Tech Spec prompt template | 同上 | 人工必審 | — |
| ⏸ | 9.5 | worker 整合生成階段 | `busy-bee-be/application/meeting/process.go` | analyzing 階段；冪等 | — |
| ⏸ | 9.6 | artifacts 查詢 API | `busy-bee-be/interface/http/handler/meeting/` | | — |

---

## Phase 10：歷史與搜尋（F-HISTORY、F-SEARCH）

> 里程碑：M2-A
> M2-A 驗收：完整會議 → 文件流程可用、可搜尋（MVP 上線）。

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ⏸ | 10.1 | meetings list/detail API | `busy-bee-be/interface/http/handler/meeting/` | 一律 user_id 過濾；待 Phase 9 | — |
| ⏸ | 10.2 | ILIKE 搜尋 API | `busy-bee-be/application/meeting/search.go` | title + transcript | — |
| ⏸ | 10.3 | 列表/詳情 UI + Markdown 渲染 | `busy-bee-fe/src/pages/` | | — |
| ⏸ | 10.4 | 搜尋 UI | `busy-bee-fe/src/components/` | | — |
| ⏸ | 10.5 | 失敗會議顯示 + 手動 retry | 前後端 | 重新 enqueue | — |

---

## Phase 11：提醒與推播（F-REMIND）

> 里程碑：M2-B

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ⏸ | 11.1 | push_subscriptions 表 + 訂閱 API | `busy-bee-be/db/migrations/`, handler | 待 Phase 4 完成（需 HTTPS 環境） | — |
| ⏸ | 11.2 | VAPID key + web-push 發送 | `busy-bee-be/infrastructure/` | private key 存 Secret Manager | — |
| ⏸ | 11.3 | SW push handler + 前端訂閱 | `busy-bee-fe/public/sw.js` | iOS 限制載明於 UI | — |
| ⏸ | 11.4 | scheduled meeting CRUD | 前後端 | scheduled 狀態的未來會議；提醒提前時間可設定（預設 15 分） | — |
| ⏸ | 11.5 | ProcessAt 延遲提醒排程 | `busy-bee-be/infrastructure/queue/` | scheduled_at 或提前時間變更時取消重排 | — |

---

## Phase 12：Production 完善

> 里程碑：M2-B

| 狀態 | # | 項目 | 檔案 | 細節 | Commit |
|------|---|------|------|------|--------|
| ⏸ | 12.1 | rate limiting middleware | `busy-bee-be/interface/http/middleware/` | 待 Phase 10 完成 | — |
| ⏸ | 12.2 | error state UI 梳理 | `busy-bee-fe/src/` | 全流程錯誤與空狀態 | — |
| ⬜ | 12.3 | Cloud Run 參數調整 | — | min-instances / CPU / 成本確認 | — |
| ⬜ | 12.4 | README + .env.example 收尾 | `README.md`, `.env.example` | | — |

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
