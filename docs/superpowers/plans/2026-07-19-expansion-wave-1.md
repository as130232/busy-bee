# 擴充第一波（行動項 / 匯出分享 / 推播深連結）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 為 Busy Bee 加入行動項抽取與追蹤（F-ACTION）、文件匯出/分享（F-EXPORT）、提醒推播深連結（F-REMIND 補強），並把擴充路線圖寫入 PRODUCT.md / PLAN.md。

**Architecture:** F-ACTION 走既有 Clean Architecture 全鏈路：analyzing 階段新增 LLM 抽取（以 artifacts 表 `action_items` JSON 為冪等標記），結構化列存入新 `action_items` 表，經 REST API 供前端 checklist 顯示與勾選。F-EXPORT 純前端（clipboard / Blob 下載 / Web Share API）。深連結：後端提醒 payload URL 帶 `?record=1`，SW notificationclick 聚焦既有分頁，Dashboard 讀 query param 讓錄音鈕發光。

**Tech Stack:** Go (Gin + sqlc/pgx + genai)、React 19 + Tailwind v4、Web Push / Service Worker。

## Global Constraints（CLAUDE.md 摘錄，全部任務適用）

- OpenAPI first：先改 `busy-bee-be/api/openapi.yaml` 再寫 handler
- 業務查詢一律以 `user_id` 過濾；repository 禁止自行開 transaction
- handler 一律走 `response.OK / response.Fail`，錯誤用 `apperr.New / Wrap`
- worker 階段冪等：已完成階段跳過（ADR-009）
- domain 層零外部依賴（僅 std + uuid 慣例已存在）
- 測試與目標檔同目錄；application 層用 fake port，不連真實 DB / API
- 命名：Go exported `MeetingID`；JSON tag `meetingId`
- 完成後更新 docs/PLAN.md

---

### Task 1: 文件更新（PRODUCT.md / PLAN.md）

**Files:**
- Modify: `docs/PRODUCT.md`（功能範圍、F-ACTION、F-EXPORT、F-REMIND 驗收、優先序、新增「擴充路線圖」節）
- Modify: `docs/PLAN.md`（進度總覽加 Phase 13、當前焦點、Phase 13 任務清單）

**內容要點：**
- PRODUCT.md 功能範圍「包含」加：F-ACTION（行動項抽取與追蹤）、F-EXPORT（文件匯出/分享）
- F-ACTION 驗收：處理完成後自動抽出行動項（描述/負責人/時限，會議未提及則空）；Dashboard 顯示未完成行動項清單（跨會議）；可勾選完成/取消完成；僅本人可見
- F-EXPORT 驗收：詳情頁可複製 Markdown、下載 .md、行動裝置可用系統分享；逐字稿同樣支援
- F-REMIND 補驗收：點提醒通知開啟 App 並聚焦既有分頁、錄音鈕高亮
- 「擴充路線圖」節：列入音訊回放對照、語意搜尋/Chat with Meetings（ADR-006）、週報 digest、錄音中重點標記、蜂巢統計頁（皆標 backlog）
- PLAN.md：Phase 13「擴充第一波」＋任務清單（對應本計畫 Task 2-10）

- [ ] Step 1: 依上述要點編輯兩份文件
- [ ] Step 2: Commit：`docs: add F-ACTION/F-EXPORT specs, expansion roadmap, Phase 13 plan`

---

### Task 2: action_items migration + sqlc

**Files:**
- Create: `busy-bee-be/db/migrations/000005_create_action_items.up.sql` / `.down.sql`
- Create: `busy-bee-be/db/query/action_items.sql`
- Generate: `busy-bee-be/infrastructure/db/sqlcgen/`（`cd busy-bee-be/db && sqlc generate`）

**Interfaces:**
- Produces: sqlcgen `ActionItem` model、`InsertActionItem`、`DeleteActionItemsForMeeting`、`ListActionItemsByMeeting`、`ListPendingActionItemsForUser`（含 meeting_title）、`SetActionItemDone`

**up.sql：**

```sql
CREATE TABLE action_items (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    meeting_id  uuid NOT NULL REFERENCES meetings (id) ON DELETE CASCADE,
    user_id     uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    description text NOT NULL,
    assignee    text NOT NULL DEFAULT '',
    due_text    text NOT NULL DEFAULT '',
    done        boolean NOT NULL DEFAULT false,
    sort_order  int NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX action_items_user_pending_idx ON action_items (user_id, done, created_at);
CREATE INDEX action_items_meeting_idx ON action_items (meeting_id);

-- artifacts 允許 action_items 類型（原始 JSON 作為抽取階段的冪等標記）
ALTER TABLE artifacts DROP CONSTRAINT artifacts_type_check;
ALTER TABLE artifacts ADD CONSTRAINT artifacts_type_check
    CHECK (artifact_type IN ('prd', 'tech_spec', 'action_items'));
```

**down.sql：**

```sql
ALTER TABLE artifacts DROP CONSTRAINT artifacts_type_check;
ALTER TABLE artifacts ADD CONSTRAINT artifacts_type_check
    CHECK (artifact_type IN ('prd', 'tech_spec'));
DROP TABLE action_items;
```

**action_items.sql：**

```sql
-- name: InsertActionItem :one
INSERT INTO action_items (meeting_id, user_id, description, assignee, due_text, sort_order)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: DeleteActionItemsForMeeting :exec
DELETE FROM action_items WHERE meeting_id = $1;

-- name: ListActionItemsByMeeting :many
SELECT * FROM action_items WHERE meeting_id = $1 ORDER BY sort_order, created_at;

-- name: ListPendingActionItemsForUser :many
SELECT ai.*, m.title AS meeting_title
FROM action_items ai
JOIN meetings m ON m.id = ai.meeting_id
WHERE ai.user_id = $1 AND ai.done = false
ORDER BY ai.created_at DESC
LIMIT 100;

-- name: SetActionItemDone :one
UPDATE action_items
SET done = $3, updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;
```

- [ ] Step 1: 寫 migration 與 query 檔
- [ ] Step 2: `cd busy-bee-be/db && sqlc generate`；`cd .. && go build ./...` 通過
- [ ] Step 3: 本地 DB 跑 migration（`go run ./cmd/migrate` 或既有方式）驗證 up/down
- [ ] Step 4: Commit：`feat: add action_items table and queries (Phase 13.1)`

---

### Task 3: domain/actionitem + Gemini 抽取 + prompt

**Files:**
- Create: `busy-bee-be/domain/actionitem/actionitem.go`
- Create: `busy-bee-be/infrastructure/llm/prompts/action_items.md`
- Modify: `busy-bee-be/infrastructure/llm/gemini.go`（實作 Extractor，含 JSON 解析）
- Create: `busy-bee-be/infrastructure/llm/gemini_test.go`（`parseActionItems` 單元測試）
- Create: `busy-bee-be/infrastructure/db/actionitem_repo.go`

**Interfaces（Produces）:**

```go
// domain/actionitem/actionitem.go
package actionitem

type ActionItem struct {
    ID          uuid.UUID
    MeetingID   uuid.UUID
    UserID      uuid.UUID
    Description string
    Assignee    string
    DueText     string
    Done        bool
    CreatedAt   time.Time
}

// PendingItem 跨會議待辦清單用（含會議標題）。
type PendingItem struct {
    ActionItem
    MeetingTitle string
}

// Extracted LLM 抽取結果（尚未落庫）。
type Extracted struct {
    Description string `json:"description"`
    Assignee    string `json:"assignee"`
    DueText     string `json:"due"`
}

type Repository interface {
    Insert(ctx context.Context, meetingID, userID uuid.UUID, item Extracted, sortOrder int) (ActionItem, error)
    DeleteForMeeting(ctx context.Context, meetingID uuid.UUID) error
    ListByMeeting(ctx context.Context, meetingID uuid.UUID) ([]ActionItem, error)
    ListPendingForUser(ctx context.Context, userID uuid.UUID) ([]PendingItem, error)
    SetDone(ctx context.Context, id, userID uuid.UUID, done bool) (ActionItem, error)
}

type Extractor interface {
    ExtractActionItems(ctx context.Context, transcript string) ([]Extracted, error)
}
```

`SetDone` 查無列（非本人或不存在）回傳 `ErrNotFound`（domain sentinel error，同 `domainmeeting.ErrNotFound` 模式）。

**prompt（prompts/action_items.md）：** 指示模型從逐字稿抽取行動項，輸出**純 JSON array**（無 markdown fence）：`[{"description": "...", "assignee": "...", "due": "..."}]`；沒有行動項輸出 `[]`；禁止捏造；assignee / due 會議未提及留空字串。

**gemini.go 新增：**

```go
func (c *GeminiClient) ExtractActionItems(ctx context.Context, transcript string) ([]domainactionitem.Extracted, error) {
    text, err := c.generate(ctx, promptActionItems, transcript)  // 復用既有 generate；空回應視為錯誤
    if err != nil { return nil, err }
    return parseActionItems(text)
}

// parseActionItems 容忍模型包 ```json fence；解析失敗回錯誤（交由 retry）。
func parseActionItems(text string) ([]domainactionitem.Extracted, error) {
    s := strings.TrimSpace(text)
    s = strings.TrimPrefix(s, "```json")
    s = strings.TrimPrefix(s, "```")
    s = strings.TrimSuffix(strings.TrimSpace(s), "```")
    var items []domainactionitem.Extracted
    if err := json.Unmarshal([]byte(strings.TrimSpace(s)), &items); err != nil {
        return nil, fmt.Errorf("llm.parseActionItems: %w", err)
    }
    return items, nil
}
```

注意：`generate` 對空字串回錯誤，但空陣列合法 → prompt 要求沒有項目時輸出 `[]`（非空字串，不觸發 empty response 錯誤）。

**actionitem_repo.go：** 依 `artifact_repo.go` 樣式包 sqlcgen，`var _ domainactionitem.Repository = (*ActionItemRepo)(nil)`；`SetDone` 的 `pgx.ErrNoRows` 轉 `domainactionitem.ErrNotFound`。

- [ ] Step 1: 寫 `gemini_test.go`：`parseActionItems` 三案例（純 JSON / fence 包裹 / 壞 JSON 回錯）→ 跑測試失敗
- [ ] Step 2: 實作 domain、prompt、gemini、repo → `go test ./infrastructure/llm/` 通過
- [ ] Step 3: `go build ./...` 通過
- [ ] Step 4: Commit：`feat: add action item domain, LLM extractor, repo (Phase 13.2)`

---

### Task 4: ProcessUC 抽取階段（TDD）

**Files:**
- Modify: `busy-bee-be/application/meeting/process.go`
- Modify: `busy-bee-be/application/meeting/process_test.go`
- Modify: `busy-bee-be/cmd/server/main.go`（ProcessDeps 注入）

**Interfaces:**
- Consumes: Task 3 的 `domainactionitem.Repository` / `Extractor`
- Produces: `ProcessDeps` 新欄位 `ActionItems domainactionitem.Repository`、`Extractor domainactionitem.Extractor`

**process.go 變更：** `generateArtifacts` 後、`SetCompleted` 前新增：

```go
// analyzing 階段：抽取行動項；artifacts 表的 action_items JSON 為冪等標記
if m.Status == domainmeeting.StatusAnalyzing {
    if err := uc.extractActionItems(ctx, m); err != nil {
        return err
    }
}
```

```go
const artifactTypeActionItems = domainartifact.Type("action_items")

func (uc *ProcessUC) extractActionItems(ctx context.Context, m domainmeeting.Meeting) error {
    existing, err := uc.artifacts.ListByMeeting(ctx, m.ID)
    if err != nil { return fmt.Errorf("process list artifacts for action items: %w", err) }
    for _, a := range existing {
        if a.Type == artifactTypeActionItems { return nil } // 冪等：已抽取
    }
    items, err := uc.extractor.ExtractActionItems(ctx, m.Transcript)
    if err != nil { return fmt.Errorf("process extract action items: %w", err) }
    // 先清舊列再逐筆插入；最後寫入 JSON 標記（中途失敗 → retry 重抽，最多重付一次 LLM）
    if err := uc.actionItems.DeleteForMeeting(ctx, m.ID); err != nil {
        return fmt.Errorf("process clear action items: %w", err)
    }
    for i, it := range items {
        if _, err := uc.actionItems.Insert(ctx, m.ID, m.UserID, it, i); err != nil {
            return fmt.Errorf("process insert action item: %w", err)
        }
    }
    raw, _ := json.Marshal(items)
    if _, err := uc.artifacts.Upsert(ctx, m.ID, artifactTypeActionItems, string(raw)); err != nil {
        return fmt.Errorf("process mark action items extracted: %w", err)
    }
    slog.InfoContext(ctx, "meeting.process.action_items_saved", "meeting_id", m.ID, "count", len(items))
    return nil
}
```

**測試（先寫、先跑失敗）：** 依既有 fake 樣式加 `processFakeActionItemRepo` / `processFakeExtractor`：
1. 正常流程：analyzing → extractor 被呼叫、items 落庫、artifacts 出現 action_items 標記、最終 completed
2. 冪等：artifacts 已有 action_items → extractor 不被呼叫
3. 空陣列：extractor 回 `[]` → 不插列、仍寫標記、completed
4. 抽取失敗：extractor 回錯 → Execute 回錯、不 completed

- [ ] Step 1: 更新 process_test.go（新增 fakes + 4 測試）→ `go test ./application/meeting/` 失敗
- [ ] Step 2: 實作 process.go 變更 → 測試通過
- [ ] Step 3: main.go 注入 `actionItemRepo := db.NewActionItemRepo(pool)` 與 llmClient（Extractor）→ `go build ./...`
- [ ] Step 4: `go test ./...` 全過
- [ ] Step 5: Commit：`feat: extract action items in pipeline (Phase 13.3)`

---

### Task 5: 行動項 API（OpenAPI → UC → handler → route）

**Files:**
- Modify: `busy-bee-be/api/openapi.yaml`
- Create: `busy-bee-be/application/actionitem/list.go`、`toggle.go`、`toggle_test.go`
- Create: `busy-bee-be/interface/http/handler/actionitem/handler.go`、`response.go`
- Modify: `busy-bee-be/interface/http/server.go`、`cmd/server/main.go`

**OpenAPI 新增：**
- `GET /api/v1/meetings/{id}/action-items` → `{ actionItems: ActionItem[] }`（404 非本人）
- `GET /api/v1/action-items` → `{ actionItems: PendingActionItem[] }`（query `done=false` 固定語意：僅回未完成）
- `PATCH /api/v1/action-items/{id}` body `{ done: boolean }` → `{ actionItem: ActionItem }`（404 非本人）
- Schema `ActionItem`：`id, meetingId, description, assignee, dueText, done, createdAt`；`PendingActionItem` 多 `meetingTitle`

**Use cases（application/actionitem）：**

```go
// list.go
type ListByMeetingUC struct { meetings domainmeeting.Repository; items domainactionitem.Repository }
// Execute(ctx, userID, meetingID)：先 meetings.GetForUser 驗所有權（同 ListArtifactsUC），再 items.ListByMeeting
type ListPendingUC struct { items domainactionitem.Repository }
// Execute(ctx, userID)：items.ListPendingForUser

// toggle.go
type ToggleUC struct { items domainactionitem.Repository }
// Execute(ctx, userID, itemID, done)：items.SetDone；ErrNotFound → apperr.New(errcode.NotFound)
```

**toggle_test.go：** fake repo 驗證 (1) done 正常更新回傳、(2) 非本人（fake 回 ErrNotFound）→ apperr NotFound。

**Handler（interface/http/handler/actionitem）：** 依 meeting handler 樣式：`IDFrom` 取 userID、`uuid.Parse` 路徑參數失敗回 `errcode.Param`、`response.OK(c, gin.H{"actionItems": ...})`。response.go 做 domain → JSON 映射（`dueText` tag）。

**Route（server.go）：**

```go
authed.GET("/meetings/:id/action-items", deps.ActionItemHandler.ListByMeeting)
authed.GET("/action-items", deps.ActionItemHandler.ListPending)
authed.PATCH("/action-items/:id", deps.ActionItemHandler.Toggle)
```

`Deps` 加 `ActionItemHandler *actionitemhandler.Handler`；main.go 組裝。

- [ ] Step 1: 改 openapi.yaml
- [ ] Step 2: 寫 toggle_test.go → 失敗
- [ ] Step 3: 實作 UC / handler / route / main 注入 → `go test ./...` 全過、`go build ./...`
- [ ] Step 4: Commit：`feat: action items API (Phase 13.4)`

---

### Task 6: FE 行動項 UI（client + Dashboard 待辦卡 + 詳情 tab）

**Files:**
- Generate: `busy-bee-fe/src/services/api/schema.d.ts`（`npm run gen:api`）
- Modify: `busy-bee-fe/src/services/api/client.ts`
- Create: `busy-bee-fe/src/components/ActionItemList.tsx`
- Modify: `busy-bee-fe/src/pages/DashboardPage.tsx`、`busy-bee-fe/src/pages/MeetingDetailPage.tsx`

**client.ts 新增：**

```ts
export type ActionItem = components['schemas']['ActionItem']
export type PendingActionItem = components['schemas']['PendingActionItem']

export function listMeetingActionItems(idToken: string, meetingId: string): Promise<{ actionItems: ActionItem[] }> {
  return request(`/api/v1/meetings/${meetingId}/action-items`, { method: 'GET' }, idToken)
}
export function listPendingActionItems(idToken: string): Promise<{ actionItems: PendingActionItem[] }> {
  return request('/api/v1/action-items', { method: 'GET' }, idToken)
}
export function toggleActionItem(idToken: string, id: string, done: boolean): Promise<{ actionItem: ActionItem }> {
  return request(`/api/v1/action-items/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ done }),
  }, idToken)
}
```

**ActionItemList 元件：** 接 `items` + `onToggle(id, done)` + 可選 `showMeetingTitle`；每列：圓形 checkbox（勾選後文字劃線淡出）、描述、`assignee · dueText` 次行（空則略）、`showMeetingTitle` 時顯示會議標題連結。樂觀更新、失敗回滾。觸控目標 ≥44px。

**DashboardPage：** 搜尋框上方加「待辦行動項」surface 卡（有未完成項才顯示）：載入 `listPendingActionItems`，顯示前 5 筆 + 「還有 N 項」；勾選即 toggle 並從清單移除。

**MeetingDetailPage：** `Tab` 型別加 `'action_items'`（標籤「行動項」，tabs 改 `grid-cols-4`）；選中時載入 `listMeetingActionItems` 顯示 `ActionItemList`（空清單顯示「本場會議沒有行動項」）。

- [ ] Step 1: `npm run gen:api`；加 client 函式
- [ ] Step 2: 實作 ActionItemList + 兩頁整合
- [ ] Step 3: `npm run typecheck && npm run lint && npm run build` 全過
- [ ] Step 4: Commit：`feat: action items UI (Phase 13.5)`

---

### Task 7: 文件匯出 / 分享（FE only）

**Files:**
- Create: `busy-bee-fe/src/components/ExportBar.tsx`
- Modify: `busy-bee-fe/src/pages/MeetingDetailPage.tsx`

**ExportBar：** props `{ content: string; filename: string }`，三個 icon 按鈕（lucide `Copy` / `Download` / `Share2`）：
- 複製：`navigator.clipboard.writeText(content)`，成功後按鈕短暫變 `Check` + 「已複製」
- 下載：`new Blob([content], { type: 'text/markdown' })` + 暫時 `<a download>` 點擊
- 分享：僅 `navigator.share` 存在時顯示；`navigator.share({ title: filename, text: content })`；`AbortError`（用戶取消）靜默

**整合：** 詳情頁 article 上方右對齊；`content` 為當前 tab 內容、`filename` 為 `${meeting.title}-${tab}.md`；內容為空或行動項 tab 時不顯示。

- [ ] Step 1: 實作 ExportBar + 整合
- [ ] Step 2: `npm run typecheck && npm run lint && npm run build` 全過
- [ ] Step 3: Commit：`feat: export/copy/share for documents (Phase 13.6)`

---

### Task 8: 提醒推播深連結

**Files:**
- Modify: `busy-bee-be/application/meeting/reminder.go`（`URL: "/?record=1"`）
- Modify: `busy-bee-be/application/meeting/reminder_test.go`（若有 fake sender 記錄 msg，補斷言 URL）
- Modify: `busy-bee-fe/src/sw.ts`（notificationclick 聚焦既有分頁）
- Modify: `busy-bee-fe/src/pages/DashboardPage.tsx`、`busy-bee-fe/src/components/RecorderPanel.tsx`（highlight prop）

**sw.ts notificationclick 改為：**

```ts
self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const url = (event.notification.data as { url?: string } | undefined)?.url ?? '/'
  event.waitUntil(
    (async () => {
      const wins = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
      const existing = wins.find((w) => new URL(w.url).origin === self.location.origin)
      if (existing) {
        await existing.navigate(url)
        await existing.focus()
        return
      }
      await self.clients.openWindow(url)
    })(),
  )
})
```

**DashboardPage：** `useSearchParams` 讀 `record=1` → 傳 `highlight` 給 RecorderPanel，並在 3 秒後 `setSearchParams({}, { replace: true })` 清除。

**RecorderPanel：** `highlight?: boolean` prop；待機大圓鈕加 `animate-pulse ring-4 ring-accent/40`（highlight 時）。

- [ ] Step 1: 後端 URL 變更 + 測試斷言 → `go test ./application/meeting/` 過
- [ ] Step 2: sw.ts / Dashboard / RecorderPanel 變更 → FE 三檢通過
- [ ] Step 3: Commit：`feat: reminder push deep link focuses app and highlights recorder (Phase 13.7)`

---

### Task 9: 端到端驗證與收尾

- [ ] Step 1: `cd busy-bee-be && go test ./...` 全過
- [ ] Step 2: `docker compose up -d` + migration + `go run ./cmd/server`；`npm run dev`；上傳短音訊跑完管線，確認 action_items 落庫、Dashboard 待辦卡與詳情 tab 顯示、勾選持久化
- [ ] Step 3: 詳情頁驗證複製 / 下載 /（手機）分享
- [ ] Step 4: 排一場 1 分鐘後的會議驗證推播點擊 → 聚焦 + 錄音鈕高亮
- [ ] Step 5: PLAN.md Phase 13 標 ✅ 填 commit hash、Session Log 補記錄
- [ ] Step 6: Commit：`docs: mark Phase 13 tasks complete`

## Self-Review 紀錄

- Spec coverage：F-ACTION（Task 2-6）、F-EXPORT（Task 7）、深連結（Task 8）、文件（Task 1、9）✓
- 空行動項冪等：以 artifacts JSON 標記解決，prompt 要求空陣列輸出 `[]` 避開 empty-response 錯誤 ✓
- 型別一致性：`Extracted{Description, Assignee, DueText}` 貫穿 Task 3-5；JSON tag `dueText` ✓
- 交易規範：無 repo 內 tx；delete→insert→marker 順序保證重試正確性（代價：極端情況多付一次 LLM）✓
