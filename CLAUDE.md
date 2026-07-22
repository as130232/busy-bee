# Busy Bee 開發指南

## 文件分工

| 文件 | 放什麼 |
|------|--------|
| `CLAUDE.md` | 規則、禁止事項、SOP（每次自動載入） |
| `docs/PRODUCT.md` | 產品需求、F-ID、Milestone、驗收條件 |
| `docs/PLAN.md` | Phase 任務清單、當前進度、Session Log |
| `docs/ARCHITECTURE.md` | 設計細節、ADR、資料流、程式碼範例 |

> 超過 5 行且含程式碼或大型表格 → 移入 ARCHITECTURE.md，此處留一行 reference。

---

## 架構

Clean Architecture（對齊 sport-hub），依賴方向由外往內：interface / worker → application → domain ← infrastructure。
詳見 `docs/ARCHITECTURE.md §2-§3`。

- 禁止 domain 層 import 任何外部套件或其他層
- Transaction boundary 一律在 application 層（WithTx），repository 禁止自行開 transaction

---

## Monorepo 邊界

| 目錄 | 內容 |
|------|------|
| `busy-bee-be/` | Go 後端（Gin + sqlc/pgx + in-process 佇列） |
| `busy-bee-fe/` | React PWA（Vite） |

---

## 新增功能流程（由底層往上）

> 需求不明確 → 先查 `docs/PRODUCT.md`；每步完成後更新 `docs/PLAN.md` 對應任務狀態。

1. `busy-bee-be/api/openapi.yaml` — 先定義 endpoint spec（OpenAPI first）
2. `pkg/consts/errcode/` — 新增業務錯誤碼（若有）
3. `domain/<feature>/` — entity / port interface
4. `infrastructure/<module>/` — interface 實作
5. `application/<feature>/` — use case
6. `interface/http/handler/<feature>/` — request / response / handler
7. `interface/http/route/` — 掛載路由
8. `busy-bee-fe/src/services/api/` — 重新生成 TS client
9. 更新 `docs/PLAN.md`：標 ✅、填 commit hash、更新「當前焦點」

---

## 判斷要動哪個檔案

| 情況 | 只動哪裡 |
|------|---------|
| 換 STT / LLM 供應商 | `infrastructure/stt/` 或 `infrastructure/llm/`（不動 domain / application）。現行 STT = Deepgram nova-2 + `zh-TW`（語者分離）；中文設定坑與 diarization 決策見 `docs/ARCHITECTURE.md` ADR-011 |
| 新增業務錯誤碼 | `pkg/consts/errcode/` |
| DB schema 變更 | `db/migrations/` + `db/query/*.sql` + 重跑 `sqlc generate` |
| 新增 HTTP endpoint | `api/openapi.yaml` → handler → route |
| prompt 調整 | `infrastructure/llm/prompts/`（不動 client 邏輯） |
| 新增紀錄情境（如面試） | 新增 `prompts/summary_<scenario>.md` + 註冊 `gemini.go` 的 `scenarioPrompts` + `domain/meeting` 的 `Scenario` 常數與 CHECK migration（不動前端渲染器/管線，B-lite，見 `docs/ARCHITECTURE.md` ADR-012） |

---

## Response 規範

- 所有 handler 回傳一律透過 `response` package，禁止直接呼叫 `c.JSON`
- 成功用 `response.OK`；失敗用 `response.Fail` 包裝 `apperr.New / Wrap`
- 外部錯誤的原始 cause 只進 log，禁止暴露給用戶端

---

## 錯誤碼規範

- 業務錯誤碼一律定義於 `pkg/consts/errcode/`，禁止魔術數字
- 包外部錯誤一律用 `apperr.Wrap`，禁止裸 `fmt.Errorf` 跨層傳遞

---

## 資料安全規範

- meetings / artifacts 查詢一律以 `user_id` 過濾，禁止無 owner 條件的業務查詢
- 禁止 log 用戶 token 與 transcript 全文（log 只記 meeting_id 等識別欄位）
- API keys（Groq / Gemini / VAPID private key）一律走 Secret Manager，禁止進 repo 或 log

---

## Worker 規範

- worker 每階段開始前必須檢查 meeting status 與產物是否已存在，已完成階段一律跳過（冪等，ADR-009）
- 狀態變更必須發布到 in-process notifier（F-STATUS 依賴此事件，ADR-010 單 instance）
- 新增 goroutine 必須有明確退出條件（context cancel 或 channel close），禁止裸 `go func()`

---

## WebSocket 規範

- WS 連線在第一則 JWT 驗證訊息通過前，禁止綁定 user、禁止推送任何資料（ADR-002）

---

## 命名規範

| 場景 | 正確 | 錯誤 |
|------|------|------|
| exported | `MeetingID` | `MeetingId` |
| unexported | `meetingID` | `meetingId` |
| JSON tag | `meetingId` | `meeting_id` |

---

## 測試規範

- 單元測試與目標檔同目錄，命名 `<file>_test.go`
- domain / application 層測試一律用 mock port interface，禁止連真實 DB / 外部 API
- 不得 commit `t.Skip` 或被註解掉的測試

---

## 常用指令

```bash
docker compose up -d      # 本地 PostgreSQL
go run ./cmd/server       # busy-bee-be/：啟動 HTTP + worker
go test ./...             # 測試
sqlc generate             # SQL 變更後重新產生
npm run dev               # busy-bee-fe/：前端開發
```

---

## 文件維護規則

- 規則、禁止事項、SOP 變動 → 更新 **CLAUDE.md**
- 設計細節、ADR、流程圖 → 更新 **docs/ARCHITECTURE.md**（Phase 完成後移除「計畫中」標註）
- 任務完成、Phase 狀態改變 → 更新 **docs/PLAN.md**（標 ✅、填 commit hash、更新「當前焦點」）
- 需求確認、驗收條件變更 → 更新 **docs/PRODUCT.md**（Q 項有答案時標註答案與日期）
- 同一類型錯誤出現兩次 → 在 CLAUDE.md 補對應的禁止規則
