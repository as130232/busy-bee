# AI 錄音重新校正（F-CORRECT）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 使用者對已完成紀錄按「AI 校正」，Gemini 聽已上傳錄音逐段修正逐字稿、重產摘要、合併行動項，可整份還原，每場最多 2 次。

**Architecture:** 新增 `transcript_corrections` 表（每次校正一筆，含快照與斷點 progress）；獨立記憶體佇列 + sweeper 跑 `ExecuteUC`；純函式負責切段、patch 驗證/套用、行動項合併；跨表原子寫入沿用 `infrastructure/db.ProcessRepo` 的 `pgx.BeginFunc` 模式。前端新增 `CorrectionPanel` 與段落「AI 已校正」標籤。

**Tech Stack:** Go（Gin、sqlc/pgx、google.golang.org/genai v1.64、ffmpeg）、PostgreSQL、React + Vite + Vitest、openapi-typescript。

**Spec:** `docs/superpowers/specs/2026-10-10-ai-transcript-correction-design.md`

## Global Constraints

- 每場最多 2 次校正；按下即計次（含失敗）。
- 還原：segments / transcript / summary_sections / summary 回到快照；行動項與 tags 不動。
- 校正不改講者代號與時間錨點，只改 `Text`。
- `manuallyEdited=true` 的段落 Gemini 不得修改（但作為上下文提供）。
- 切段約 12 分鐘（`chunkTargetMs = 720_000`）、對齊 segment 邊界；音訊片段往前重疊 30 秒（`overlapMs = 30_000`）；上下文帶上一段最後 3 個 segment。
- patch 驗證：新文字長度 / 原文長度 ∈ [0.5, 2.0]，否則丟棄。
- 建議提示門檻：`uncertainRatio >= 0.05`。
- 所有 handler 回傳走 `response` package；錯誤用 `apperr.New/Wrap`；外部 cause 只進 log。
- 查詢一律帶 `user_id`；不 log 逐字稿全文。
- 新 goroutine 必須有 ctx cancel 退出條件。
- 整合測試加 `//go:build integration`；預設 `go test ./...` 只跑單元測試。
- JSON tag camelCase；Go exported `MeetingID` 風格。

## Review Focus

1. **校正進行中使用者編輯段落**：使用者的編輯必須優先 → Task 4 `TestApplyPatches_SkipsSegmentsEditedDuringCorrection`。
2. **同一 patch 第二次套用（重試/斷點）不得覆蓋最初原文** → Task 4 `TestApplyPatches_KeepsFirstOriginalText`。
3. **Gemini 回傳越界 index、鎖定段、空字串、暴增文字** → Task 4 `TestValidatePatches_*`。
4. **行動項：勾選後取消勾選（done=false 但 edited 由回填判定）、手動項、描述只差標點的重複項** → Task 5 `TestMergeActionItems_*`。
5. **斷點續跑：progress 已有的段不得再呼叫 Gemini** → Task 8 `TestExecute_SkipsCompletedChunks`。

---

## File Structure

| 檔案 | 責任 |
|------|------|
| `busy-bee-be/domain/meeting/stt.go` | `TranscriptSegment` 加 `ManuallyEdited`、`OriginalText`；`UncertainRatio` |
| `busy-bee-be/domain/correction/correction.go` | `Correction` entity、`Status`、`Patch`、`Chunk`、`Snapshot`、ports |
| `busy-bee-be/domain/actionitem/actionitem.go` | `ActionItem` 加 `Source`、`Edited` |
| `busy-bee-be/db/migrations/000017_transcript_corrections.{up,down}.sql` | 新表 + `action_items.edited` + 回填 |
| `busy-bee-be/db/query/corrections.sql` | correction CRUD |
| `busy-bee-be/infrastructure/db/correction_repo.go` | `CorrectionRepo`（含原子交易方法） |
| `busy-bee-be/infrastructure/stt/clip.go` | `CutClip`（ffmpeg 剪片段） |
| `busy-bee-be/infrastructure/llm/correct.go` + `prompts/correct_transcript.md` | `CorrectChunk` |
| `busy-bee-be/application/correction/chunks.go` | `PlanChunks` |
| `busy-bee-be/application/correction/patches.go` | `ValidatePatches`、`ApplyPatches` |
| `busy-bee-be/application/correction/merge.go` | `MergeActionItems` |
| `busy-bee-be/application/correction/start.go` / `execute.go` / `revert.go` / `get.go` | use cases |
| `busy-bee-be/worker/sweeper.go` | 泛化 lister 介面供 correction 共用 |
| `busy-bee-be/interface/http/handler/correction/` | handler + response |
| `busy-bee-fe/src/components/CorrectionPanel.tsx` | 按鈕、狀態、還原 |
| `busy-bee-fe/src/components/TranscriptEditor.tsx` | 「AI 已校正」標籤 + 原文 Sheet |

---

### Task 1: 段落欄位、低信心比例、手動編輯標記

**Files:**
- Modify: `busy-bee-be/domain/meeting/stt.go`
- Test: `busy-bee-be/domain/meeting/stt_test.go`（不存在則建立）
- Modify: `busy-bee-be/application/meeting/edit_transcript.go`、`edit_transcript_test.go`
- Modify: `busy-bee-be/interface/http/handler/meeting/response.go`、`response_test.go`
- Modify: `busy-bee-be/api/openapi.yaml`（`TranscriptSegment`、`MeetingDetail`）

**Interfaces:**
- Produces: `TranscriptSegment.ManuallyEdited bool`（`json:"manuallyEdited,omitempty"`）、`TranscriptSegment.OriginalText string`（`json:"originalText,omitempty"`）、`func UncertainRatio(segs []TranscriptSegment) float64`

- [ ] **Step 1: 寫失敗測試** `domain/meeting/stt_test.go`

```go
func TestUncertainRatio(t *testing.T) {
	segs := []TranscriptSegment{
		{Text: "預算三百萬", MarkedText: "預算〔?三百萬〕"}, // 3/5 不確定
		{Text: "好的好的好"},                           // 0/5
	}
	if got := UncertainRatio(segs); got < 0.299 || got > 0.301 {
		t.Errorf("UncertainRatio = %v, want 0.3", got)
	}
	if got := UncertainRatio(nil); got != 0 {
		t.Errorf("nil = %v, want 0", got)
	}
}
```

- [ ] **Step 2: 執行確認失敗** — `cd busy-bee-be && go test ./domain/meeting/ -run TestUncertainRatio` → FAIL（undefined）

- [ ] **Step 3: 實作** `domain/meeting/stt.go`（在 `MarkedText` 欄位後加）

```go
	// ManuallyEdited 使用者手動修正過此段；AI 校正時只讀不改。
	ManuallyEdited bool `json:"manuallyEdited,omitempty"`
	// OriginalText AI 校正修改此段時保存的校正前文字；空代表未被校正修改。
	OriginalText string `json:"originalText,omitempty"`
```

```go
// UncertainRatio 回傳逐字稿中低信心字元（MarkedText 內「〔?…〕」包住者）佔總字元比例（0–1）。
// 以 Text 的 rune 數為分母；無段落或無文字回 0。
func UncertainRatio(segs []TranscriptSegment) float64 {
	total, uncertain := 0, 0
	for _, s := range segs {
		total += len([]rune(s.Text))
		in := false
		for _, r := range s.MarkedText {
			switch {
			case r == '〔':
				in = true
			case r == '〕':
				in = false
			case in && r != '?':
				uncertain++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(uncertain) / float64(total)
}
```

- [ ] **Step 4: 執行確認通過** — 同 Step 2 → PASS

- [ ] **Step 5: 編輯段落設旗標** — `edit_transcript_test.go` 的 `TestEditSegment_UpdatesTextAndReflattens` 加斷言：

```go
	if !repo.savedSegs[0].ManuallyEdited {
		t.Error("人工編輯後應標記 ManuallyEdited")
	}
```

執行 `go test ./application/meeting/ -run TestEditSegment` → FAIL；在 `edit_transcript.go` 的 `segs[index].MarkedText = ""` 下一行加：

```go
	segs[index].ManuallyEdited = true // AI 校正時不得覆蓋人工修正
```

再執行 → PASS。

- [ ] **Step 6: API 輸出** — `response.go`：`segmentResponse` 加欄位

```go
	ManuallyEdited bool   `json:"manuallyEdited"`
	OriginalText   string `json:"originalText,omitempty"`
```

`toMeetingDetailResponse` 的 segment 映射加 `ManuallyEdited: s.ManuallyEdited, OriginalText: s.OriginalText`；`meetingDetailResponse` 加 `UncertainRatio float64 \`json:"uncertainRatio"\``，賦值 `domainmeeting.UncertainRatio(m.TranscriptSegments)`。`response_test.go` 加：

```go
func TestToMeetingDetailResponse_CorrectionFields(t *testing.T) {
	m := domainmeeting.Meeting{TranscriptSegments: []domainmeeting.TranscriptSegment{
		{Speaker: "A", Text: "好", MarkedText: "〔?好〕", ManuallyEdited: true, OriginalText: "號"},
	}}
	r := toMeetingDetailResponse(m)
	if !r.TranscriptSegments[0].ManuallyEdited || r.TranscriptSegments[0].OriginalText != "號" {
		t.Errorf("segment = %+v", r.TranscriptSegments[0])
	}
	if r.UncertainRatio != 1 {
		t.Errorf("UncertainRatio = %v, want 1", r.UncertainRatio)
	}
}
```

- [ ] **Step 7: OpenAPI** — `TranscriptSegment` schema 加 `manuallyEdited: {type: boolean}`（required）、`originalText: {type: string}`；`MeetingDetail` 加 `uncertainRatio: {type: number, format: double}`（required）。

- [ ] **Step 8: 全測 + commit**

```bash
cd busy-bee-be && go vet ./... && go test ./...
git add -A && git commit -m "feat(correction): 段落 manuallyEdited/originalText 欄位與低信心比例"
```

---

### Task 2: Migration、sqlc、行動項 Source/Edited

**Files:**
- Create: `busy-bee-be/db/migrations/000017_transcript_corrections.up.sql`、`.down.sql`
- Create: `busy-bee-be/db/query/corrections.sql`
- Modify: `busy-bee-be/db/query/action_items.sql`（`UpdateActionItemDescription`）
- Modify: `busy-bee-be/domain/actionitem/actionitem.go`、`infrastructure/db/actionitem_repo.go`
- Test: `busy-bee-be/infrastructure/db/correction_repo_integration_test.go`（Task 6 補齊）

**Interfaces:**
- Produces: `ActionItem.Source string`（`"llm"`/`"manual"`）、`ActionItem.Edited bool`；sqlc `sqlcgen.TranscriptCorrection` 與 queries：`InsertCorrection`、`GetLatestCorrectionForMeeting`、`GetCorrection`、`CountCorrectionsForMeeting`、`UpdateCorrectionStatus`、`UpdateCorrectionProgress`、`ListUnfinishedCorrectionIDs`

- [ ] **Step 1: up migration**

```sql
CREATE TABLE transcript_corrections (
    id                        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    meeting_id                uuid NOT NULL REFERENCES meetings (id) ON DELETE CASCADE,
    user_id                   uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    attempt                   int  NOT NULL CHECK (attempt BETWEEN 1 AND 2),
    status                    text NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'completed', 'failed', 'reverted')),
    progress                  jsonb NOT NULL DEFAULT '{"chunks":{}}',
    original_segments         jsonb NOT NULL,
    original_transcript       text  NOT NULL,
    original_summary_sections jsonb NOT NULL,
    original_summary          text  NOT NULL,
    error_message             text  NOT NULL DEFAULT '',
    created_at                timestamptz NOT NULL DEFAULT now(),
    completed_at              timestamptz,
    reverted_at               timestamptz,
    UNIQUE (meeting_id, attempt)
);
CREATE UNIQUE INDEX transcript_corrections_active_idx
    ON transcript_corrections (meeting_id) WHERE status IN ('queued', 'running');
CREATE INDEX transcript_corrections_unfinished_idx
    ON transcript_corrections (status) WHERE status IN ('queued', 'running');

-- 行動項是否被使用者改過內容；AI 校正重抽時保留 edited 項。
ALTER TABLE action_items ADD COLUMN edited boolean NOT NULL DEFAULT false;
-- 舊資料保守回填：曾被更新過（改內容、勾選又取消、寄過提醒）一律視為改過，寧可多留不誤刪。
UPDATE action_items SET edited = true WHERE updated_at > created_at;
```

down：

```sql
ALTER TABLE action_items DROP COLUMN edited;
DROP TABLE transcript_corrections;
```

- [ ] **Step 2: queries** `db/query/corrections.sql`

```sql
-- name: InsertCorrection :one
INSERT INTO transcript_corrections (
    meeting_id, user_id, attempt, original_segments, original_transcript,
    original_summary_sections, original_summary)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetCorrection :one
SELECT * FROM transcript_corrections WHERE id = $1;

-- name: GetLatestCorrectionForMeeting :one
SELECT * FROM transcript_corrections
WHERE meeting_id = $1 AND user_id = $2
ORDER BY attempt DESC LIMIT 1;

-- name: CountCorrectionsForMeeting :one
SELECT count(*) FROM transcript_corrections WHERE meeting_id = $1 AND user_id = $2;

-- name: UpdateCorrectionStatus :one
UPDATE transcript_corrections
SET status = sqlc.arg(to_status),
    error_message = sqlc.arg(error_message),
    completed_at = CASE WHEN sqlc.arg(to_status)::text = 'completed' THEN now() ELSE completed_at END,
    reverted_at  = CASE WHEN sqlc.arg(to_status)::text = 'reverted'  THEN now() ELSE reverted_at END
WHERE id = $1 AND status = sqlc.arg(from_status)
RETURNING *;

-- name: UpdateCorrectionProgress :exec
UPDATE transcript_corrections SET progress = $2 WHERE id = $1;

-- name: ListUnfinishedCorrectionIDs :many
SELECT id FROM transcript_corrections WHERE status IN ('queued', 'running');
```

`action_items.sql` 的 `UpdateActionItemDescription` 改為 `SET description = $3, edited = true, updated_at = now()`。

- [ ] **Step 3: 產生** — `cd busy-bee-be && sqlc generate && go build ./...`

- [ ] **Step 4: domain** — `ActionItem` 加：

```go
	// Source 來源：llm（AI 抽取）/ manual（使用者新增）。
	Source string
	// Edited 使用者改過內容（AI 校正重抽時保留）。
	Edited bool
```

`toDomainActionItem` 映射 `Source: row.Source, Edited: row.Edited`。

- [ ] **Step 5: 套 migration 驗證** — `docker compose up -d && go run ./cmd/migrate up && go run ./cmd/migrate down 1 && go run ./cmd/migrate up`（指令以專案現有 `cmd/migrate` 為準），確認無錯誤。

- [ ] **Step 6: 全測 + commit**

```bash
go vet ./... && go test ./...
git add -A && git commit -m "feat(correction): migration 000017、corrections queries、行動項 edited 欄位"
```

---

### Task 3: domain/correction 與錯誤碼

**Files:**
- Create: `busy-bee-be/domain/correction/correction.go`、`correction_test.go`
- Modify: `busy-bee-be/pkg/consts/errcode/errcode.go`、`msg.go`、`errcode_test.go`

**Interfaces:**
- Produces（後續 Task 全部依賴，名稱不可改）：

```go
package correction

type Status string
const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusReverted  Status = "reverted"
)
const MaxAttempts = 2

// Patch 一段逐字稿的修正：I 為 meeting.TranscriptSegments 的全域索引。
type Patch struct {
	I    int    `json:"i"`
	Text string `json:"text"`
}

// Chunk 一個處理段：[First, Last] 為 segment 全域索引（含），音訊範圍 [ClipFromMs, ClipToMs]。
type Chunk struct {
	Index      int
	First      int
	Last       int
	ClipFromMs int
	ClipToMs   int
}

// Progress 斷點：已完成段索引（字串化）→ 該段通過驗證的 patches。
type Progress struct {
	Chunks map[string][]Patch `json:"chunks"`
}

type Snapshot struct {
	Segments        []meeting.TranscriptSegment
	Transcript      string
	SummarySections []meeting.SummarySection
	Summary         string
}

type Correction struct {
	ID           uuid.UUID
	MeetingID    uuid.UUID
	UserID       uuid.UUID
	Attempt      int
	Status       Status
	Progress     Progress
	Original     Snapshot
	ErrorMessage string
	CreatedAt    time.Time
	CompletedAt  *time.Time
	RevertedAt   *time.Time
}

var (
	ErrNotFound       = errors.New("correction: not found")
	ErrStatusConflict = errors.New("correction: status conflict")
	ErrActiveExists   = errors.New("correction: active correction exists")
	// ErrBadOutput 模型輸出無法解析；ExecuteUC 視為該段無修改（不重試、不扣額外費用）。
	ErrBadOutput = errors.New("correction: bad model output")
)

// CorrectorInput 一段送給 LLM 的內容。
type CorrectorInput struct {
	Audio    io.Reader
	Filename string
	Context  []meeting.TranscriptSegment // 上一段結尾（唯讀上下文）
	Segments []IndexedSegment
}
type IndexedSegment struct {
	I       int
	Segment meeting.TranscriptSegment
	Locked  bool
}

// Corrector 聽音訊片段修正逐字稿，只回傳有修改的段落。
type Corrector interface {
	CorrectChunk(ctx context.Context, in CorrectorInput) ([]Patch, error)
}

// ClipCutter 從完整音檔剪出 [fromMs, toMs] 片段（本機暫存檔路徑）。
type ClipCutter interface {
	CutClip(ctx context.Context, srcPath string, fromMs, toMs int) (clipPath string, cleanup func(), err error)
}

// Repository correction 存取；所有讀取以 userID 過濾（worker 的 Get 例外，以 correction ID 取用）。
type Repository interface {
	// CreateWithSnapshot 原子：計算 attempt、寫快照、insert queued。超過 MaxAttempts 回 ErrStatusConflict；已有進行中回 ErrActiveExists。
	CreateWithSnapshot(ctx context.Context, meetingID, userID uuid.UUID, snap Snapshot) (Correction, error)
	Get(ctx context.Context, id uuid.UUID) (Correction, error)
	GetLatestForMeeting(ctx context.Context, meetingID, userID uuid.UUID) (Correction, error)
	CountForMeeting(ctx context.Context, meetingID, userID uuid.UUID) (int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, from, to Status, errorMessage string) (Correction, error)
	SaveProgress(ctx context.Context, id uuid.UUID, p Progress) error
	ListUnfinishedIDs(ctx context.Context) ([]uuid.UUID, error)
	// ApplyResult 原子：寫 segments/transcript、summary_sections、summary、行動項替換、status→completed。
	ApplyResult(ctx context.Context, c Correction, r Result) error
	// Revert 原子：快照寫回 meeting、status completed→reverted。
	Revert(ctx context.Context, c Correction) error
}

// Result 校正完成要落庫的產物。
type Result struct {
	Segments        []meeting.TranscriptSegment
	Transcript      string
	SummarySections []meeting.SummarySection
	Summary         string
	DeleteItemIDs   []uuid.UUID
	InsertItems     []actionitem.Extracted
	InsertSortFrom  int
}

// State 對外狀態（API 用）。
type State struct {
	Status       Status
	AttemptsUsed int
	CanRevert    bool
	ErrorMessage string
}
```

- [ ] **Step 1: 失敗測試** `correction_test.go`

```go
func TestStateFromLatest(t *testing.T) {
	cases := []struct {
		name   string
		latest *Correction
		count  int
		want   State
	}{
		{"無紀錄", nil, 0, State{}},
		{"完成可還原", &Correction{Status: StatusCompleted}, 1, State{Status: StatusCompleted, AttemptsUsed: 1, CanRevert: true}},
		{"已還原不可再還原", &Correction{Status: StatusReverted}, 2, State{Status: StatusReverted, AttemptsUsed: 2}},
		{"失敗帶訊息", &Correction{Status: StatusFailed, ErrorMessage: "x"}, 1, State{Status: StatusFailed, AttemptsUsed: 1, ErrorMessage: "x"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := StateFromLatest(c.latest, c.count); got != c.want {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: 確認失敗** — `go test ./domain/correction/`

- [ ] **Step 3: 實作** 上方 Interfaces 全部型別，加：

```go
// StateFromLatest 由最新一筆與總筆數推導對外狀態。
func StateFromLatest(latest *Correction, count int) State {
	if latest == nil {
		return State{}
	}
	return State{
		Status:       latest.Status,
		AttemptsUsed: count,
		CanRevert:    latest.Status == StatusCompleted,
		ErrorMessage: latest.ErrorMessage,
	}
}
```

（domain 只 import 標準庫、`github.com/google/uuid` 與同層 `domain/meeting`、`domain/actionitem`，與既有 domain 套件依賴一致。）

- [ ] **Step 4: 錯誤碼** — `errcode.go` 加並映射到 409：

```go
	CorrectionLimitReached ErrCode = 40902
	CorrectionInProgress   ErrCode = 40903
	CorrectionNotAllowed   ErrCode = 40904
	NothingToRevert        ErrCode = 40905
```

`msg.go`：`"correction limit reached"`、`"correction already in progress"`、`"meeting cannot be corrected"`、`"nothing to revert"`。`errcode_test.go` 的表格加四列 `http.StatusConflict`。

- [ ] **Step 5: 全測 + commit** — `go test ./domain/... ./pkg/...`；`git commit -m "feat(correction): domain entity、ports 與錯誤碼"`

---

### Task 4: 切段與 patch 純函式

**Files:**
- Create: `busy-bee-be/application/correction/chunks.go`、`chunks_test.go`
- Create: `busy-bee-be/application/correction/patches.go`、`patches_test.go`

**Interfaces:**
- Consumes: `correction.Chunk`、`correction.Patch`、`meeting.TranscriptSegment`
- Produces:
  - `func PlanChunks(segs []meeting.TranscriptSegment) []correction.Chunk`
  - `func ValidatePatches(segs []meeting.TranscriptSegment, ch correction.Chunk, in []correction.Patch) []correction.Patch`
  - `func ApplyPatches(current []meeting.TranscriptSegment, patches []correction.Patch) []meeting.TranscriptSegment`
  - 常數 `chunkTargetMs = 720_000`、`overlapMs = 30_000`、`contextSegments = 3`

- [ ] **Step 1: 失敗測試** `chunks_test.go`

```go
func seg(startMs, endMs int) meeting.TranscriptSegment {
	return meeting.TranscriptSegment{Speaker: "A", Text: "x", StartMs: startMs, EndMs: endMs}
}

func TestPlanChunks_SplitsAtSegmentBoundaries(t *testing.T) {
	// 每段 5 分鐘、共 5 段（25 分鐘）。累積區間（首段起點～末段終點）不超過 12 分鐘：
	// [0,1]=10 分、加 2 會變 15 分 → 切；[2,3]=10 分；[4]。非首段音訊往前重疊 30 秒。
	segs := []meeting.TranscriptSegment{seg(0, 300_000), seg(300_000, 600_000), seg(600_000, 900_000), seg(900_000, 1_200_000), seg(1_200_000, 1_500_000)}
	got := PlanChunks(segs)
	want := []correction.Chunk{
		{Index: 0, First: 0, Last: 1, ClipFromMs: 0, ClipToMs: 600_000},
		{Index: 1, First: 2, Last: 3, ClipFromMs: 570_000, ClipToMs: 1_200_000},
		{Index: 2, First: 4, Last: 4, ClipFromMs: 1_170_000, ClipToMs: 1_500_000},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestPlanChunks_SingleLongSegmentIsOwnChunk(t *testing.T) {
	got := PlanChunks([]meeting.TranscriptSegment{seg(0, 1_000_000), seg(1_000_000, 1_010_000)})
	if len(got) != 2 || got[0].First != 0 || got[0].Last != 0 || got[1].First != 1 {
		t.Errorf("got %+v", got)
	}
}

func TestPlanChunks_Empty(t *testing.T) {
	if got := PlanChunks(nil); len(got) != 0 {
		t.Errorf("got %+v", got)
	}
}
```

- [ ] **Step 2: 確認失敗** — `go test ./application/correction/ -run TestPlanChunks`

- [ ] **Step 3: 實作** `chunks.go`

```go
// Package correction AI 錄音重新校正 use cases 與純函式（切段、patch、行動項合併）。
package correction

const (
	chunkTargetMs   = 720_000 // 每段約 12 分鐘
	overlapMs       = 30_000  // 音訊片段往前重疊，讓模型聽到跨段語句開頭
	contextSegments = 3       // 上一段結尾帶入的唯讀段落數
)

// PlanChunks 依 segment 時間切段：累積到「再加下一個 segment 會超過 chunkTargetMs」即切，
// 切點必在 segment 邊界；單一 segment 超長時自成一段。
func PlanChunks(segs []meeting.TranscriptSegment) []correction.Chunk {
	var out []correction.Chunk
	first := 0
	for i := range segs {
		isLast := i == len(segs)-1
		if !isLast && segs[i+1].EndMs-segs[first].StartMs <= chunkTargetMs {
			continue
		}
		from := segs[0].StartMs // 第一段無前文可重疊
		if first > 0 {
			from = max(0, segs[first].StartMs-overlapMs)
		}
		out = append(out, correction.Chunk{
			Index: len(out), First: first, Last: i,
			ClipFromMs: from, ClipToMs: segs[i].EndMs,
		})
		first = i + 1
	}
	return out
}
```

- [ ] **Step 4: 確認通過** — `go test ./application/correction/ -run TestPlanChunks` → PASS

- [ ] **Step 5: 失敗測試** `patches_test.go`

```go
func TestValidatePatches(t *testing.T) {
	segs := []meeting.TranscriptSegment{
		{Text: "小娃處理"}, {Text: "接流串接", ManuallyEdited: true}, {Text: "今年預算"}, {Text: "外段"},
	}
	ch := correction.Chunk{First: 0, Last: 2}
	in := []correction.Patch{
		{I: 0, Text: "小王處理"},           // 保留
		{I: 1, Text: "金流串接"},           // 鎖定 → 丟
		{I: 2, Text: "今年預算"},           // 與原文相同 → 丟
		{I: 3, Text: "外段改"},            // 越界 → 丟
		{I: 0, Text: ""},                // 空 → 丟
		{I: 2, Text: "今年預算是三百萬元整加上其他費用"}, // 長度比 > 2 → 丟
	}
	got := ValidatePatches(segs, ch, in)
	want := []correction.Patch{{I: 0, Text: "小王處理"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestApplyPatches_SetsOriginalAndClearsMark(t *testing.T) {
	cur := []meeting.TranscriptSegment{{Speaker: "B", Text: "小娃處理", MarkedText: "〔?小娃〕處理", StartMs: 5}}
	got := ApplyPatches(cur, []correction.Patch{{I: 0, Text: "小王處理"}})
	s := got[0]
	if s.Text != "小王處理" || s.OriginalText != "小娃處理" || s.MarkedText != "" || s.Speaker != "B" || s.StartMs != 5 {
		t.Errorf("got %+v", s)
	}
	if cur[0].Text != "小娃處理" {
		t.Error("不可修改輸入 slice")
	}
}

func TestApplyPatches_KeepsFirstOriginalText(t *testing.T) {
	cur := []meeting.TranscriptSegment{{Text: "小王處理", OriginalText: "小娃處理"}}
	got := ApplyPatches(cur, []correction.Patch{{I: 0, Text: "小王來處理"}})
	if got[0].OriginalText != "小娃處理" {
		t.Errorf("OriginalText = %q，應保留最初原文", got[0].OriginalText)
	}
}

func TestApplyPatches_SkipsSegmentsEditedDuringCorrection(t *testing.T) {
	cur := []meeting.TranscriptSegment{{Text: "使用者剛改的", ManuallyEdited: true}}
	got := ApplyPatches(cur, []correction.Patch{{I: 0, Text: "AI 版本"}})
	if got[0].Text != "使用者剛改的" || got[0].OriginalText != "" {
		t.Errorf("got %+v", got[0])
	}
}

func TestApplyPatches_IgnoresOutOfRange(t *testing.T) {
	got := ApplyPatches([]meeting.TranscriptSegment{{Text: "a"}}, []correction.Patch{{I: 5, Text: "b"}})
	if got[0].Text != "a" {
		t.Errorf("got %+v", got)
	}
}
```

- [ ] **Step 6: 確認失敗** — `go test ./application/correction/ -run 'Patches'`

- [ ] **Step 7: 實作** `patches.go`

```go
const (
	minLenRatio = 0.5
	maxLenRatio = 2.0
)

// ValidatePatches 丟棄不合法的 patch：越界、鎖定段、空字串、與原文相同、長度比超出 [0.5, 2.0]。
func ValidatePatches(segs []meeting.TranscriptSegment, ch correction.Chunk, in []correction.Patch) []correction.Patch {
	out := make([]correction.Patch, 0, len(in))
	for _, p := range in {
		text := strings.TrimSpace(p.Text)
		if p.I < ch.First || p.I > ch.Last || p.I >= len(segs) {
			continue
		}
		orig := segs[p.I]
		if orig.ManuallyEdited || text == "" || text == orig.Text {
			continue
		}
		ratio := float64(len([]rune(text))) / float64(max(1, len([]rune(orig.Text))))
		if ratio < minLenRatio || ratio > maxLenRatio {
			continue
		}
		out = append(out, correction.Patch{I: p.I, Text: text})
	}
	return out
}

// ApplyPatches 回傳套用後的新 slice（不改輸入）：
// 此刻已 ManuallyEdited 的段跳過（校正期間使用者編輯優先）；OriginalText 只在首次被改時寫入；清空 MarkedText。
func ApplyPatches(current []meeting.TranscriptSegment, patches []correction.Patch) []meeting.TranscriptSegment {
	out := slices.Clone(current)
	for _, p := range patches {
		if p.I < 0 || p.I >= len(out) || out[p.I].ManuallyEdited {
			continue
		}
		s := &out[p.I]
		if s.OriginalText == "" {
			s.OriginalText = s.Text
		}
		s.Text = p.Text
		s.MarkedText = ""
	}
	return out
}
```

- [ ] **Step 8: 確認通過 + commit** — `go test ./application/correction/`；`git commit -m "feat(correction): 切段與 patch 驗證/套用純函式"`

---

### Task 5: 行動項合併與 Extractor 排除既有項

**Files:**
- Create: `busy-bee-be/application/correction/merge.go`、`merge_test.go`
- Modify: `busy-bee-be/domain/actionitem/actionitem.go`（`Extractor` 加方法）
- Modify: `busy-bee-be/infrastructure/llm/gemini.go`、`prompts/action_items.md`、`gemini_test.go`
- Modify: `busy-bee-be/application/meeting/process_test.go` 的 `fakeExtractor`（實作新方法）

**Interfaces:**
- Produces:
  - `type MergePlan struct { DeleteIDs []uuid.UUID; Insert []actionitem.Extracted; InsertSortFrom int }`
  - `func SplitActionItems(items []actionitem.ActionItem) (kept, replaceable []actionitem.ActionItem)`
  - `func PlanMerge(kept, replaceable []actionitem.ActionItem, extracted []actionitem.Extracted) MergePlan`
  - `actionitem.Extractor` 新增 `ExtractExcluding(ctx context.Context, transcript string, now time.Time, existing []string) (Extraction, error)`

- [ ] **Step 1: 失敗測試** `merge_test.go`

```go
func item(desc, source string, done, edited bool) actionitem.ActionItem {
	return actionitem.ActionItem{ID: uuid.New(), Description: desc, Source: source, Done: done, Edited: edited}
}

func TestSplitActionItems(t *testing.T) {
	manual := item("手動", "manual", false, false)
	done := item("已勾", "llm", true, false)
	edited := item("改過", "llm", false, true)
	plain := item("原樣", "llm", false, false)
	kept, repl := SplitActionItems([]actionitem.ActionItem{manual, done, edited, plain})
	if len(kept) != 3 || len(repl) != 1 || repl[0].ID != plain.ID {
		t.Errorf("kept=%v repl=%v", kept, repl)
	}
}

func TestPlanMerge_DedupesAgainstKept(t *testing.T) {
	kept := []actionitem.ActionItem{item("整理規格書。", "manual", false, false), item("寄信", "llm", true, false)}
	repl := []actionitem.ActionItem{item("舊的", "llm", false, false)}
	ext := []actionitem.Extracted{{Description: "整理 規格書"}, {Description: "確認現金流"}}
	got := PlanMerge(kept, repl, ext)
	if len(got.DeleteIDs) != 1 || got.DeleteIDs[0] != repl[0].ID {
		t.Errorf("DeleteIDs = %v", got.DeleteIDs)
	}
	if len(got.Insert) != 1 || got.Insert[0].Description != "確認現金流" {
		t.Errorf("Insert = %+v（只差空白/標點者應去重）", got.Insert)
	}
	if got.InsertSortFrom != 2 {
		t.Errorf("InsertSortFrom = %d, want 2（接在保留項後）", got.InsertSortFrom)
	}
}
```

- [ ] **Step 2: 確認失敗**

- [ ] **Step 3: 實作** `merge.go`

```go
type MergePlan struct {
	DeleteIDs      []uuid.UUID
	Insert         []actionitem.Extracted
	InsertSortFrom int
}

// SplitActionItems 手動新增 / 已勾選 / 改過內容 → 保留；其餘 AI 抽取項 → 可覆蓋。
func SplitActionItems(items []actionitem.ActionItem) (kept, replaceable []actionitem.ActionItem) {
	for _, it := range items {
		if it.Source == "manual" || it.Done || it.Edited {
			kept = append(kept, it)
		} else {
			replaceable = append(replaceable, it)
		}
	}
	return kept, replaceable
}

// PlanMerge 刪可覆蓋項、插入新抽取項（與保留項正規化後相同者略過），新項排在保留項之後。
func PlanMerge(kept, replaceable []actionitem.ActionItem, extracted []actionitem.Extracted) MergePlan {
	seen := make(map[string]bool, len(kept))
	for _, k := range kept {
		seen[normalizeDesc(k.Description)] = true
	}
	plan := MergePlan{InsertSortFrom: len(kept)}
	for _, r := range replaceable {
		plan.DeleteIDs = append(plan.DeleteIDs, r.ID)
	}
	for _, e := range extracted {
		key := normalizeDesc(e.Description)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		plan.Insert = append(plan.Insert, e)
	}
	return plan
}

// normalizeDesc 去除空白與標點、轉小寫，供去重比較。
func normalizeDesc(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
```

- [ ] **Step 4: 確認通過**

- [ ] **Step 5: Extractor 擴充** — `actionitem.Extractor` 加 `ExtractExcluding`；`gemini.go`：

```go
// ExtractExcluding 同 Extract，但告知模型 existing 中的行動項已存在、不得重複輸出（AI 校正重抽用）。
func (c *GeminiClient) ExtractExcluding(ctx context.Context, transcript string, now time.Time, existing []string) (domainactionitem.Extraction, error) {
	prompt, err := buildPrompt(promptActionItems, transcript)
	if err != nil {
		return domainactionitem.Extraction{}, err
	}
	prompt = strings.ReplaceAll(prompt, "{{TODAY}}", now.Format("2006-01-02"))
	prompt = strings.ReplaceAll(prompt, "{{EXISTING}}", existingBlock(existing))
	text, err := c.complete(ctx, promptActionItems, prompt)
	if err != nil {
		return domainactionitem.Extraction{}, err
	}
	return parseExtraction(text)
}

// existingBlock 既有行動項清單；無則回「（無）」。
func existingBlock(existing []string) string {
	if len(existing) == 0 {
		return "（無）"
	}
	var b strings.Builder
	for _, e := range existing {
		fmt.Fprintf(&b, "- %s\n", e)
	}
	return strings.TrimSpace(b.String())
}
```

`Extract` 改為呼叫 `c.ExtractExcluding(ctx, transcript, now, nil)`。`prompts/action_items.md` 在「若逐字稿沒有任何行動項」那行後加：

```markdown
- 以下行動項**已存在**，不得重複輸出（意思相同換句話說也算重複）：
{{EXISTING}}
```

`gemini_test.go` 加：

```go
func TestExistingBlock(t *testing.T) {
	if existingBlock(nil) != "（無）" {
		t.Error("空清單應為（無）")
	}
	if got := existingBlock([]string{"a", "b"}); got != "- a\n- b" {
		t.Errorf("got %q", got)
	}
}
```

`process_test.go` 的 `fakeExtractor` 加：

```go
func (f *fakeExtractor) ExtractExcluding(ctx context.Context, tr string, now time.Time, _ []string) (domainactionitem.Extraction, error) {
	return f.Extract(ctx, tr, now)
}
```

- [ ] **Step 6: 全測 + commit** — `go vet ./... && go test ./...`；`git commit -m "feat(correction): 行動項合併規則與 ExtractExcluding"`

---

### Task 6: CorrectionRepo（含原子交易）

**Files:**
- Create: `busy-bee-be/infrastructure/db/correction_repo.go`
- Create: `busy-bee-be/infrastructure/db/correction_repo_integration_test.go`（`//go:build integration`）

**Interfaces:**
- Consumes: Task 2 sqlc queries；Task 3 `correction.Repository`
- Produces: `func NewCorrectionRepo(pool *pgxpool.Pool) *CorrectionRepo`（實作 `correction.Repository`）

- [ ] **Step 1: 實作骨架與映射**

```go
// CorrectionRepo 實作 domain/correction.Repository。
// 跨表原子操作沿用 ProcessRepo 作法（pgx.BeginFunc）。
type CorrectionRepo struct {
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

var _ domaincorrection.Repository = (*CorrectionRepo)(nil)

func NewCorrectionRepo(pool *pgxpool.Pool) *CorrectionRepo {
	return &CorrectionRepo{pool: pool, q: sqlcgen.New(pool)}
}
```

`toDomainCorrection(row sqlcgen.TranscriptCorrection) (domaincorrection.Correction, error)`：`json.Unmarshal` progress / original_segments / original_summary_sections；`Progress.Chunks` 為 nil 時初始化為空 map。

- [ ] **Step 2: `CreateWithSnapshot`**

```go
func (r *CorrectionRepo) CreateWithSnapshot(ctx context.Context, meetingID, userID uuid.UUID, snap domaincorrection.Snapshot) (domaincorrection.Correction, error) {
	segJSON, err := json.Marshal(snap.Segments)
	if err != nil {
		return domaincorrection.Correction{}, fmt.Errorf("db.CreateCorrection marshal segments: %w", err)
	}
	secJSON, err := json.Marshal(snap.SummarySections)
	if err != nil {
		return domaincorrection.Correction{}, fmt.Errorf("db.CreateCorrection marshal sections: %w", err)
	}
	var out domaincorrection.Correction
	err = pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)
		n, err := q.CountCorrectionsForMeeting(ctx, sqlcgen.CountCorrectionsForMeetingParams{MeetingID: meetingID, UserID: userID})
		if err != nil {
			return fmt.Errorf("db.CreateCorrection count: %w", err)
		}
		if n >= domaincorrection.MaxAttempts {
			return domaincorrection.ErrStatusConflict
		}
		row, err := q.InsertCorrection(ctx, sqlcgen.InsertCorrectionParams{
			MeetingID: meetingID, UserID: userID, Attempt: int32(n) + 1,
			OriginalSegments: segJSON, OriginalTranscript: snap.Transcript,
			OriginalSummarySections: secJSON, OriginalSummary: snap.Summary,
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return domaincorrection.ErrActiveExists // 部分唯一索引或 attempt 併發衝突
			}
			return fmt.Errorf("db.CreateCorrection insert: %w", err)
		}
		out, err = toDomainCorrection(row)
		return err
	})
	return out, err
}
```

- [ ] **Step 3: 單表方法** — `Get`、`GetLatestForMeeting`（`pgx.ErrNoRows` → `ErrNotFound`，沿用 `mapNoRows`）、`CountForMeeting`、`UpdateStatus`（no rows → `ErrStatusConflict`）、`SaveProgress`（marshal 後 `UpdateCorrectionProgress`）、`ListUnfinishedIDs`。

- [ ] **Step 4: `ApplyResult`**

```go
func (r *CorrectionRepo) ApplyResult(ctx context.Context, c domaincorrection.Correction, res domaincorrection.Result) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)
		meetings := &MeetingRepo{q: q}
		items := &ActionItemRepo{q: q}
		if _, err := meetings.UpdateTranscriptSegments(ctx, c.MeetingID, c.UserID, res.Segments, res.Transcript); err != nil {
			return fmt.Errorf("db.ApplyCorrection segments: %w", err)
		}
		if len(res.SummarySections) > 0 {
			if _, err := meetings.SaveSummarySections(ctx, c.MeetingID, c.UserID, res.SummarySections); err != nil {
				return fmt.Errorf("db.ApplyCorrection sections: %w", err)
			}
		}
		if res.Summary != "" {
			if _, err := meetings.SaveSummary(ctx, c.MeetingID, c.UserID, res.Summary); err != nil {
				return fmt.Errorf("db.ApplyCorrection summary: %w", err)
			}
		}
		for _, id := range res.DeleteItemIDs {
			if err := items.Delete(ctx, id, c.UserID); err != nil && !errors.Is(err, domainactionitem.ErrNotFound) {
				return fmt.Errorf("db.ApplyCorrection delete item: %w", err)
			}
		}
		for i, it := range res.InsertItems {
			if _, err := items.Insert(ctx, c.MeetingID, c.UserID, it, res.InsertSortFrom+i); err != nil {
				return fmt.Errorf("db.ApplyCorrection insert item: %w", err)
			}
		}
		if _, err := q.UpdateCorrectionStatus(ctx, sqlcgen.UpdateCorrectionStatusParams{
			ID: c.ID, FromStatus: string(domaincorrection.StatusRunning), ToStatus: string(domaincorrection.StatusCompleted),
		}); err != nil {
			return mapNoRows(err, domaincorrection.ErrStatusConflict, "db.ApplyCorrection status")
		}
		return nil
	})
}
```

（`ErrNotFound` 刪除容忍：使用者在校正期間自行刪掉該項。）

- [ ] **Step 5: `Revert`** — 同 transaction：`UpdateTranscriptSegments(snap.Segments, snap.Transcript)`、`SaveSummarySections(snap.SummarySections)`、`SaveSummary(snap.Summary)`（空字串也寫入，需確認 `SaveSummary` 允許空值；不允許則新增 query `RestoreMeetingSummary`）、`UpdateCorrectionStatus completed→reverted`。

- [ ] **Step 6: 整合測試**（`//go:build integration`，沿用 `db_test.go` 的測試 DB 建立方式）

```go
func TestCorrectionRepo_CreateLimitsAndActive(t *testing.T) {
	// 建 user + completed meeting（沿用既有 helper）
	// 1. CreateWithSnapshot → attempt=1, status=queued
	// 2. 再 Create → ErrActiveExists（部分唯一索引）
	// 3. UpdateStatus queued→running→failed；Create → attempt=2
	// 4. UpdateStatus 2 → failed；Create → ErrStatusConflict（上限）
}

func TestCorrectionRepo_ApplyAndRevert(t *testing.T) {
	// 1. Create（快照 segments=[{A,"小娃"}], summary="舊"）→ running
	// 2. ApplyResult(segments=[{A,"小王",OriginalText:"小娃"}], Summary="新", 刪一個 llm 項、插一個新項)
	//    → meeting.TranscriptSegments[0].Text=="小王"、Summary=="新"、行動項數正確、status=completed
	// 3. Revert → Text=="小娃"、OriginalText==""、Summary=="舊"、行動項不變、status=reverted
}
```

每個註解步驟寫成實際斷言。執行：`go test -tags integration ./infrastructure/db/ -run Correction`（需 `docker compose up -d` 並套 migration）。

- [ ] **Step 7: 單元全測 + commit** — `go test ./...`；`git commit -m "feat(correction): CorrectionRepo 與原子套用/還原"`

---

### Task 7: 音訊剪片段與 Gemini 校正 client

**Files:**
- Create: `busy-bee-be/infrastructure/stt/clip.go`、`clip_integration_test.go`（`//go:build integration`）
- Create: `busy-bee-be/infrastructure/llm/correct.go`、`correct_test.go`
- Create: `busy-bee-be/infrastructure/llm/prompts/correct_transcript.md`

**Interfaces:**
- Produces: `stt.FFmpegClipper`（實作 `correction.ClipCutter`）、`(*llm.GeminiClient).CorrectChunk`（實作 `correction.Corrector`）、`func buildCorrectionPrompt(in correction.CorrectorInput) string`、`func parsePatches(text string) ([]correction.Patch, error)`

- [ ] **Step 1: `clip.go`**

```go
// FFmpegClipper 以 ffmpeg 剪出音訊片段（16kbps mono mp3，與 CompressToMP3 同規格）。
type FFmpegClipper struct{}

func (FFmpegClipper) CutClip(ctx context.Context, srcPath string, fromMs, toMs int) (string, func(), error) {
	dir, err := os.MkdirTemp("", "bb-clip-*")
	if err != nil {
		return "", nil, fmt.Errorf("clip mkdtemp: %w", err)
	}
	cleanup := func() { os.RemoveAll(dir) }
	out := filepath.Join(dir, "clip.mp3")
	cmd := exec.CommandContext(ctx, "ffmpeg", "-ss", msToSec(fromMs), "-to", msToSec(toMs),
		"-i", srcPath, "-ac", "1", "-b:a", "16k", "-y", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("clip ffmpeg: %w: %s", err, tail(b, 300))
	}
	return out, cleanup, nil
}

func msToSec(ms int) string { return strconv.FormatFloat(float64(ms)/1000, 'f', 3, 64) }
```

整合測試：用 `ffmpeg -f lavfi -i sine=duration=10` 產 10 秒檔，剪 2000–5000ms，`ffprobe` 檢查長度約 3 秒（±0.2）。

- [ ] **Step 2: prompt** `prompts/correct_transcript.md`

```markdown
你是逐字稿校對員。附上的音檔是一段會議錄音片段，下方是語音辨識產生的逐字稿段落（可能有同音錯字、漏字；「〔?…〕」為辨識信心低的部分）。請仔細聽音檔，修正逐字稿。

規則：
- 只修正「音檔中實際說出、但逐字稿寫錯或漏掉」的文字；不得加入音檔沒有的內容、不得改寫語氣或摘要化。
- 不得變更講者與段落編號；每段只能修正該段自己的文字。
- 標記 `🔒` 的段落為使用者已人工修正，**不得輸出**。
- 「上文」僅供理解語意銜接，**不得輸出**。
- 繁體中文；英文專有名詞用正確拼寫。
- 沒有需要修改的段落時輸出 `[]`。

輸出：**只輸出 JSON 陣列**，每項 `{"i": 段落編號, "text": "修正後完整段落文字"}`，只列出有修改的段落，不要任何說明文字、不要 markdown 程式碼區塊。

上文（唯讀）：
{{CONTEXT}}

待校正段落：
{{SEGMENTS}}
```

- [ ] **Step 3: 失敗測試** `correct_test.go`

```go
func TestBuildCorrectionPrompt(t *testing.T) {
	in := correction.CorrectorInput{
		Context: []meeting.TranscriptSegment{{Speaker: "A", Text: "前文"}},
		Segments: []correction.IndexedSegment{
			{I: 7, Segment: meeting.TranscriptSegment{Speaker: "B", Text: "小娃處理", MarkedText: "〔?小娃〕處理", StartMs: 61000}},
			{I: 8, Segment: meeting.TranscriptSegment{Speaker: "A", Text: "人工"}, Locked: true},
		},
	}
	p := buildCorrectionPrompt(in)
	for _, want := range []string{"A: 前文", "[7] (01:01) B: 〔?小娃〕處理", "[8] 🔒 (00:00) A: 人工"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt 缺少 %q", want)
		}
	}
}

func TestParsePatches(t *testing.T) {
	got, err := parsePatches("```json\n[{\"i\":7,\"text\":\"小王處理\"}]\n```")
	if err != nil || len(got) != 1 || got[0].I != 7 || got[0].Text != "小王處理" {
		t.Errorf("got %+v err %v", got, err)
	}
	if _, err := parsePatches("not json"); !errors.Is(err, correction.ErrBadOutput) {
		t.Errorf("壞 JSON 應回 ErrBadOutput，got %v", err)
	}
}
```

- [ ] **Step 4: 實作** `correct.go`

```go
//go:embed prompts/correct_transcript.md
var correctPrompt string

// CorrectChunk 上傳音訊片段（Files API）＋逐字稿段落，回傳模型提出的修正（未驗證，由 application 層 ValidatePatches）。
func (c *GeminiClient) CorrectChunk(ctx context.Context, in correction.CorrectorInput) ([]correction.Patch, error) {
	file, err := c.client.Files.Upload(ctx, in.Audio, &genai.UploadFileConfig{MIMEType: "audio/mpeg"})
	if err != nil {
		return nil, fmt.Errorf("llm.CorrectChunk upload: %w", err)
	}
	defer func() { _, _ = c.client.Files.Delete(context.WithoutCancel(ctx), file.Name, nil) }()
	for file.State == genai.FileStateProcessing {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
		if file, err = c.client.Files.Get(ctx, file.Name, nil); err != nil {
			return nil, fmt.Errorf("llm.CorrectChunk poll: %w", err)
		}
	}
	if file.State == genai.FileStateFailed {
		return nil, fmt.Errorf("llm.CorrectChunk: file processing failed")
	}
	cfg := generateConfig(c.model)
	if cfg == nil {
		cfg = &genai.GenerateContentConfig{}
	}
	cfg.ResponseMIMEType = "application/json"
	contents := []*genai.Content{genai.NewContentFromParts([]*genai.Part{
		genai.NewPartFromText(buildCorrectionPrompt(in)),
		genai.NewPartFromURI(file.URI, file.MIMEType),
	}, genai.RoleUser)}
	resp, err := c.client.Models.GenerateContent(ctx, c.model, contents, cfg)
	if err != nil {
		return nil, fmt.Errorf("llm.CorrectChunk generate: %w", err)
	}
	return parsePatches(resp.Text())
}

func buildCorrectionPrompt(in correction.CorrectorInput) string {
	var ctxB, segB strings.Builder
	for _, s := range in.Context {
		fmt.Fprintf(&ctxB, "%s: %s\n", s.Speaker, s.Text)
	}
	for _, s := range in.Segments {
		lock := ""
		if s.Locked {
			lock = "🔒 "
		}
		sec := s.Segment.StartMs / 1000
		fmt.Fprintf(&segB, "[%d] %s(%02d:%02d) %s: %s\n", s.I, lock, sec/60, sec%60, s.Segment.Speaker, s.Segment.AnalysisText())
	}
	p := strings.ReplaceAll(correctPrompt, "{{CONTEXT}}", orNone(ctxB.String()))
	return strings.ReplaceAll(p, "{{SEGMENTS}}", strings.TrimSpace(segB.String()))
}

func parsePatches(text string) ([]correction.Patch, error) {
	var out []correction.Patch
	if err := json.Unmarshal([]byte(stripJSONFence(text)), &out); err != nil {
		return nil, fmt.Errorf("llm.parsePatches: %w: %v", correction.ErrBadOutput, err)
	}
	return out, nil
}
```

（`orNone`、`stripJSONFence`、`generateConfig` 皆為 `gemini.go` 既有函式。）

- [ ] **Step 5: 全測 + commit** — `go test ./infrastructure/...`；`go test -tags integration ./infrastructure/stt/ -run Clip`（需 ffmpeg）；`git commit -m "feat(correction): ffmpeg 剪片段與 Gemini CorrectChunk"`

---

### Task 8: Use cases（Start / Execute / Revert / Get）

**Files:**
- Create: `busy-bee-be/application/correction/start.go`、`execute.go`、`revert.go`、`get.go`
- Create: `busy-bee-be/application/correction/usecase_test.go`（mock ports）

**Interfaces:**
- Consumes: Task 3–7 全部
- Produces:

```go
type Queue interface { EnqueueCorrection(ctx context.Context, correctionID uuid.UUID) error }
type MeetingReader interface {
	GetForUser(ctx context.Context, id, userID uuid.UUID) (meeting.Meeting, error)
}
type ActionItemLister interface {
	ListByMeeting(ctx context.Context, meetingID uuid.UUID) ([]actionitem.ActionItem, error)
}
type Indexer interface { Execute(ctx context.Context, meetingID uuid.UUID) error }

func NewStartUC(meetings MeetingReader, repo correction.Repository, queue Queue) *StartUC
func (uc *StartUC) Execute(ctx context.Context, userID, meetingID uuid.UUID) (correction.State, error)
func NewGetUC(meetings MeetingReader, repo correction.Repository) *GetUC
func (uc *GetUC) Execute(ctx context.Context, userID, meetingID uuid.UUID) (correction.State, error)
func NewRevertUC(repo correction.Repository, indexer Indexer) *RevertUC
func (uc *RevertUC) Execute(ctx context.Context, userID, meetingID uuid.UUID) (correction.State, error)

type ExecuteDeps struct {
	Repo       correction.Repository
	Meetings   MeetingReader
	Storage    meeting.AudioStorage
	Clipper    correction.ClipCutter
	Corrector  correction.Corrector
	Summarizer meeting.Summarizer
	Extractor  actionitem.Extractor
	Items      ActionItemLister
	Indexer    Indexer
	Notifier   meeting.StatusNotifier
	Compress   func(ctx context.Context, r io.Reader) (path string, cleanup func(), err error)
}
func NewExecuteUC(d ExecuteDeps) *ExecuteUC
func (uc *ExecuteUC) Execute(ctx context.Context, correctionID uuid.UUID) error
func (uc *ExecuteUC) MarkFailed(ctx context.Context, correctionID uuid.UUID, cause error)
```

- [ ] **Step 1: StartUC 失敗測試**（`usecase_test.go` 建 `fakeRepo`、`fakeMeetings`、`fakeQueue` 等）

```go
func TestStart_RejectsNotCompleted(t *testing.T) {
	uc := NewStartUC(&fakeMeetings{m: meeting.Meeting{Status: meeting.StatusAnalyzing, AudioGCSPath: "a"}}, &fakeRepo{}, &fakeQueue{})
	_, err := uc.Execute(ctx, uid, mid)
	assertErrCode(t, err, errcode.CorrectionNotAllowed)
}
func TestStart_RejectsNoAudio(t *testing.T)      { /* AudioGCSPath="" → CorrectionNotAllowed */ }
func TestStart_LimitReached(t *testing.T)        { /* fakeRepo.createErr=ErrStatusConflict → CorrectionLimitReached */ }
func TestStart_InProgress(t *testing.T)          { /* fakeRepo.createErr=ErrActiveExists → CorrectionInProgress */ }
func TestStart_SnapshotsAndEnqueues(t *testing.T) {
	m := completedMeeting() // 有 segments、SummarySections、Summary
	repo, q := &fakeRepo{}, &fakeQueue{}
	st, err := NewStartUC(&fakeMeetings{m: m}, repo, q).Execute(ctx, uid, mid)
	// 斷言：repo.gotSnap 等於 m 的 segments/transcript/sections/summary；q.ids 含新 correction ID；st.Status==queued、AttemptsUsed==1
}
```

（`assertErrCode` 以 `errors.As(err, &apperr.Error)` 比對 code，沿用專案既有測試寫法；若無共用 helper，在本檔定義。）每個 `/* */` 依註解寫成完整測試。

- [ ] **Step 2: 實作 StartUC**

```go
func (uc *StartUC) Execute(ctx context.Context, userID, meetingID uuid.UUID) (correction.State, error) {
	m, err := uc.meetings.GetForUser(ctx, meetingID, userID)
	if err != nil {
		if errors.Is(err, meeting.ErrNotFound) {
			return correction.State{}, apperr.New(errcode.NotFound)
		}
		return correction.State{}, apperr.Wrap(err, errcode.Internal)
	}
	if m.Status != meeting.StatusCompleted || m.AudioGCSPath == "" || len(m.TranscriptSegments) == 0 {
		return correction.State{}, apperr.New(errcode.CorrectionNotAllowed)
	}
	c, err := uc.repo.CreateWithSnapshot(ctx, m.ID, userID, correction.Snapshot{
		Segments: m.TranscriptSegments, Transcript: m.Transcript,
		SummarySections: m.SummarySections, Summary: m.Summary,
	})
	switch {
	case errors.Is(err, correction.ErrStatusConflict):
		return correction.State{}, apperr.New(errcode.CorrectionLimitReached)
	case errors.Is(err, correction.ErrActiveExists):
		return correction.State{}, apperr.New(errcode.CorrectionInProgress)
	case err != nil:
		return correction.State{}, apperr.Wrap(err, errcode.Internal)
	}
	if err := uc.queue.EnqueueCorrection(ctx, c.ID); err != nil {
		// 已落庫為 queued，sweeper 會補排；不回錯讓使用者重按（會被 InProgress 擋）
		slog.WarnContext(ctx, "correction.start.enqueue_failed", "correction_id", c.ID, "err", err)
	}
	return correction.StateFromLatest(&c, c.Attempt), nil
}
```

- [ ] **Step 3: GetUC / RevertUC 測試與實作**

測試：`TestGet_NoCorrection`（`ErrNotFound` → `State{}`）、`TestRevert_OnlyLatestCompleted`（latest=failed → `NothingToRevert`）、`TestRevert_RevertsAndReindexes`（呼叫 `repo.Revert`、`indexer.Execute`，回傳 State.Status=reverted）。

```go
func (uc *RevertUC) Execute(ctx context.Context, userID, meetingID uuid.UUID) (correction.State, error) {
	c, err := uc.repo.GetLatestForMeeting(ctx, meetingID, userID)
	if errors.Is(err, correction.ErrNotFound) || (err == nil && c.Status != correction.StatusCompleted) {
		return correction.State{}, apperr.New(errcode.NothingToRevert)
	}
	if err != nil {
		return correction.State{}, apperr.Wrap(err, errcode.Internal)
	}
	if err := uc.repo.Revert(ctx, c); err != nil {
		if errors.Is(err, correction.ErrStatusConflict) {
			return correction.State{}, apperr.New(errcode.NothingToRevert)
		}
		return correction.State{}, apperr.Wrap(err, errcode.Internal)
	}
	if err := uc.indexer.Execute(ctx, meetingID); err != nil {
		slog.WarnContext(ctx, "correction.revert.reindex_failed", "meeting_id", meetingID, "err", err) // 回填掃描會補
	}
	c.Status = correction.StatusReverted
	n, _ := uc.repo.CountForMeeting(ctx, meetingID, userID)
	return correction.StateFromLatest(&c, n), nil
}
```

GetUC：先 `meetings.GetForUser`（owner 檢查），再 `GetLatestForMeeting` + `CountForMeeting` → `StateFromLatest`。

- [ ] **Step 4: ExecuteUC 失敗測試**

```go
func TestExecute_SkipsTerminal(t *testing.T)        { /* status=completed → 不呼叫 Corrector、不改狀態 */ }
func TestExecute_SkipsCompletedChunks(t *testing.T) {
	// 3 段 chunk（segments 時間跨 30 分鐘）；progress 已有 "0" → Corrector 只被呼叫 2 次（chunk 1、2）
	// 每段完成後 SaveProgress 被呼叫，最後一次含 3 個 key
}
func TestExecute_AppliesValidatedPatchesAndMerges(t *testing.T) {
	// Corrector 對 chunk0 回 [{I:0,"小王處理"},{I:1,"鎖定段改寫"}]，segment 1 ManuallyEdited
	// Summarizer 回新 sections；Extractor 回 [{整理規格書},{確認現金流}]；Items 有 manual「整理規格書」+ 一個可覆蓋 llm 項
	// 斷言 ApplyResult 收到：segments[0].Text=小王處理/OriginalText=原文；segments[1] 不變；
	//   DeleteItemIDs=[llm 項]；InsertItems=[確認現金流]；InsertSortFrom=1；Summary=Extractor.summary
	// 斷言 Extractor 收到 existing=["整理規格書"]；Indexer.Execute 被呼叫；Notifier 收到 running 與 completed 兩次事件
}
func TestExecute_CorrectorBadJSONTreatedAsNoChange(t *testing.T) {
	// Corrector 回 error 包含 parse 失敗 → 由 ExecuteUC 視為該段 [] 並記入 progress，不回錯
}
```

Corrector 的 parse 錯誤以 `errors.Is(err, correction.ErrBadOutput)` 辨識（Task 3 定義、Task 7 `parsePatches` 包裝）。

- [ ] **Step 5: 實作 ExecuteUC**

```go
func (uc *ExecuteUC) Execute(ctx context.Context, id uuid.UUID) error {
	c, err := uc.d.Repo.Get(ctx, id)
	if errors.Is(err, correction.ErrNotFound) {
		return nil // 紀錄已刪（cascade）
	}
	if err != nil {
		return fmt.Errorf("correction get: %w", err)
	}
	switch c.Status {
	case correction.StatusCompleted, correction.StatusFailed, correction.StatusReverted:
		return nil
	case correction.StatusQueued:
		if c, err = uc.d.Repo.UpdateStatus(ctx, id, correction.StatusQueued, correction.StatusRunning, ""); err != nil {
			return fmt.Errorf("correction start running: %w", err)
		}
		uc.notify(ctx, c)
	}

	m, err := uc.d.Meetings.GetForUser(ctx, c.MeetingID, c.UserID)
	if errors.Is(err, meeting.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("correction get meeting: %w", err)
	}
	// 切段以快照 segments 為準（索引穩定）；套用時才讀最新 segments。
	base := c.Original.Segments
	chunks := PlanChunks(base)

	audioPath, cleanupAudio, err := uc.prepareAudio(ctx, m.AudioGCSPath)
	if err != nil {
		return err
	}
	defer cleanupAudio()

	var all []correction.Patch
	for _, ch := range chunks {
		key := strconv.Itoa(ch.Index)
		if done, ok := c.Progress.Chunks[key]; ok {
			all = append(all, done...)
			continue
		}
		patches, err := uc.correctChunk(ctx, audioPath, base, all, ch)
		if err != nil {
			return err
		}
		c.Progress.Chunks[key] = patches
		if err := uc.d.Repo.SaveProgress(ctx, c.ID, c.Progress); err != nil {
			return fmt.Errorf("correction save progress: %w", err)
		}
		all = append(all, patches...)
	}

	m, err = uc.d.Meetings.GetForUser(ctx, c.MeetingID, c.UserID) // 重讀：校正期間的人工編輯優先
	if err != nil {
		return fmt.Errorf("correction reload meeting: %w", err)
	}
	segs := ApplyPatches(m.TranscriptSegments, all)
	transcript := meeting.FlattenSegments(segs)

	sum, err := uc.d.Summarizer.Summarize(ctx, timestamped(segs), m.Scenario)
	if err != nil {
		return fmt.Errorf("correction summarize: %w", err)
	}
	items, err := uc.d.Items.ListByMeeting(ctx, m.ID)
	if err != nil {
		return fmt.Errorf("correction list items: %w", err)
	}
	kept, repl := SplitActionItems(items)
	existing := make([]string, len(kept))
	for i, k := range kept {
		existing[i] = k.Description
	}
	ext, err := uc.d.Extractor.ExtractExcluding(ctx, analysisText(segs), m.CreatedAt, existing)
	if err != nil {
		return fmt.Errorf("correction extract: %w", err)
	}
	plan := PlanMerge(kept, repl, ext.Items)

	if err := uc.d.Repo.ApplyResult(ctx, c, correction.Result{
		Segments: segs, Transcript: transcript,
		SummarySections: sum.Sections, Summary: strings.TrimSpace(ext.Summary),
		DeleteItemIDs: plan.DeleteIDs, InsertItems: plan.Insert, InsertSortFrom: plan.InsertSortFrom,
	}); err != nil {
		return fmt.Errorf("correction apply: %w", err)
	}
	if err := uc.d.Indexer.Execute(ctx, m.ID); err != nil {
		slog.WarnContext(ctx, "correction.reindex_failed", "meeting_id", m.ID, "err", err)
	}
	c.Status = correction.StatusCompleted
	uc.notify(ctx, c)
	slog.InfoContext(ctx, "correction.completed", "meeting_id", m.ID, "correction_id", c.ID,
		"chunks", len(chunks), "patches", len(all))
	return nil
}
```

輔助：
- `prepareAudio`：`Storage.Download` → `Compress`（main 注入 `stt.CompressToMP3` 的 adapter，回傳暫存檔路徑與 cleanup）。
- `correctChunk`：`Clipper.CutClip(audioPath, ch.ClipFromMs, ch.ClipToMs)` → 開檔 → 組 `CorrectorInput`：`Context` = `ApplyPatches(base, all)` 中 `[max(0, ch.First-contextSegments), ch.First)`；`Segments` = `base[ch.First..ch.Last]` 各附 `I`、`Locked: seg.ManuallyEdited` → `Corrector.CorrectChunk`；`errors.Is(err, correction.ErrBadOutput)` → 回 `[]`（log warn）；其他錯誤回傳（佇列 retry）→ `ValidatePatches(base, ch, out)`。
- `timestamped` / `analysisText`：與 `application/meeting/process.go` 的 `timestampedTranscript` / `analysisTranscript` 相同邏輯——**先把這兩個函式移到 `domain/meeting/stt.go` 為 exported `TimestampedTranscript(segs, fallback)`、`AnalysisTranscript(segs, fallback)`**（含既有測試一併搬移），兩邊共用，避免複製。
- `notify`：`Notifier.NotifyStatus(StatusEvent{MeetingID, UserID, Status: meeting.StatusCompleted, CorrectionStatus: string(c.Status)})`。
- `MarkFailed`：`UpdateStatus(running→failed, "AI 校正失敗，請稍後再試")`（queued→failed 亦嘗試），log cause（只含 IDs），notify。

- [ ] **Step 6: StatusEvent 擴充** — `domain/meeting/notifier.go` 的 `StatusEvent` 加 `CorrectionStatus string`；`interface/http/ws/hub.go` 的 `statusMessage` 加 `CorrectionStatus string \`json:"correctionStatus,omitempty"\`` 並賦值。hub 既有測試確認不受影響。

- [ ] **Step 7: 全測 + commit** — `go vet ./... && go test ./...`；`git commit -m "feat(correction): Start/Execute/Revert/Get use cases"`

---

### Task 9: 佇列、Sweeper、main 組裝

**Files:**
- Modify: `busy-bee-be/infrastructure/queue/memory.go`（加 `EnqueueCorrection` 別名方法）
- Modify: `busy-bee-be/worker/sweeper.go`、`sweeper_test.go`
- Modify: `busy-bee-be/cmd/server/main.go`

**Interfaces:**
- Produces: `(*queue.Memory).EnqueueCorrection(ctx, id) error`（委派 `EnqueueProcessMeeting` 的同一內部 enqueue）；`worker.NewSweeperFunc(list func(ctx) ([]uuid.UUID, error), enqueue func(ctx, uuid.UUID) error) *Sweeper`

- [ ] **Step 1: Memory** — 加：

```go
// EnqueueCorrection 供 AI 校正佇列使用（同一 Memory 型別的另一個實例，各自 handler）。
func (m *Memory) EnqueueCorrection(ctx context.Context, id uuid.UUID) error {
	return m.EnqueueProcessMeeting(ctx, id)
}
```

`memory_test.go` 加一例確認 `EnqueueCorrection` 會被消費（複製 `TestMemory_EnqueueAndConsume` 改呼叫）。

- [ ] **Step 2: Sweeper 泛化** — 新增建構子，內部以函式欄位呼叫；原 `NewSweeper(lister, queue)` 改為呼叫 `NewSweeperFunc(lister.ListUnfinishedIDs, queue.EnqueueProcessMeeting)`，既有測試不變；加 `TestSweeperFunc_EnqueuesListed`。

- [ ] **Step 3: main.go** — 在 `taskQueue.Start` 之後：

```go
	// AI 校正（Phase 27）：獨立佇列，外部 API bound，併發 1；重啟由 correction sweeper 復原
	correctionRepo := db.NewCorrectionRepo(pool)
	executeCorrection := appcorrection.NewExecuteUC(appcorrection.ExecuteDeps{
		Repo: correctionRepo, Meetings: meetingRepo, Storage: audioStorage,
		Clipper: stt.FFmpegClipper{}, Corrector: llmClient, Summarizer: llmClient,
		Extractor: llmClient, Items: actionItemRepo, Indexer: indexUC, Notifier: hub,
		Compress: stt.CompressToTempFile,
	})
	correctionQueue := queue.NewMemory(64, cfg.Worker.TaskTimeout, queue.DefaultRetryDelays)
	correctionQueue.Start(ctx, 1, executeCorrection.Execute, executeCorrection.MarkFailed)
	go worker.NewSweeperFunc(correctionRepo.ListUnfinishedIDs, correctionQueue.EnqueueCorrection).Run(sweepCtx, sweepInterval)
```

關機區段在 `taskQueue.Stop` 旁加 `correctionQueue.Stop(shutdownTimeout)`。`stt.CompressToTempFile`：於 `compress.go` 新增，包 `CompressToMP3` 回傳 `(path, cleanup, err)`（`tempFileReader` 已持有暫存路徑；若未暴露路徑則加 `Path()` 方法）。`cfg.Worker.TaskTimeout` 若小於 15 分鐘，新增 `CORRECTION_TASK_TIMEOUT`（預設 `20m`）於 `infrastructure/config/config.go` 並用之。

- [ ] **Step 4: build + 全測 + commit** — `go build ./... && go test ./...`；`git commit -m "feat(correction): 校正佇列、sweeper 與組裝"`

---

### Task 10: HTTP endpoints

**Files:**
- Modify: `busy-bee-be/api/openapi.yaml`
- Create: `busy-bee-be/interface/http/handler/correction/handler.go`、`handler_test.go`
- Modify: `busy-bee-be/interface/http/server.go`（路由）、`cmd/server/main.go`（handler 注入）

**Interfaces:**
- Produces: `GET /api/v1/meetings/{id}/correction`、`POST /api/v1/meetings/{id}/correction`、`POST /api/v1/meetings/{id}/correction/revert`，皆回 `{"correction": CorrectionState}`

- [ ] **Step 1: OpenAPI** — 新 schema：

```yaml
    CorrectionState:
      type: object
      required: [status, attemptsUsed, attemptsMax, canRevert, errorMessage]
      properties:
        status: {type: string, enum: ["", queued, running, completed, failed, reverted]}
        attemptsUsed: {type: integer}
        attemptsMax: {type: integer}
        canRevert: {type: boolean}
        errorMessage: {type: string}
```

三個 path（JWT security 同既有 meetings 路徑；回應 envelope 照 `/meetings/{id}/retry` 的寫法），錯誤 409 列出四個新 errcode。

- [ ] **Step 2: handler 失敗測試**（沿用 `handler/meeting/handler_test.go` 的 gin test 寫法與 fake UC 介面）

```go
func TestStart_Returns200WithState(t *testing.T)       { /* fake start 回 State{queued,1} → 200，body.data.correction.status=="queued"、attemptsMax==2 */ }
func TestStart_LimitReachedMaps409(t *testing.T)       { /* fake 回 apperr.New(CorrectionLimitReached) → 409、errCode 40902 */ }
func TestGet_InvalidIDIs400(t *testing.T)              { /* id="abc" → 400 */ }
```

- [ ] **Step 3: 實作**

```go
type startUC interface {
	Execute(ctx context.Context, userID, meetingID uuid.UUID) (correction.State, error)
}

type Handler struct{ get, start, revert startUC }

func NewHandler(get, start, revert startUC) *Handler { return &Handler{get: get, start: start, revert: revert} }

type stateResponse struct {
	Status       string `json:"status"`
	AttemptsUsed int    `json:"attemptsUsed"`
	AttemptsMax  int    `json:"attemptsMax"`
	CanRevert    bool   `json:"canRevert"`
	ErrorMessage string `json:"errorMessage"`
}

func toStateResponse(s correction.State) gin.H {
	return gin.H{"correction": stateResponse{
		Status: string(s.Status), AttemptsUsed: s.AttemptsUsed, AttemptsMax: correction.MaxAttempts,
		CanRevert: s.CanRevert, ErrorMessage: s.ErrorMessage,
	}}
}

func (h *Handler) Get(c *gin.Context)    { h.run(c, h.get) }
func (h *Handler) Start(c *gin.Context)  { h.run(c, h.start) }
func (h *Handler) Revert(c *gin.Context) { h.run(c, h.revert) }

func (h *Handler) run(c *gin.Context, uc startUC) {
	// userID / meetingID 解析沿用 handler/meeting/manage.go Retry 的寫法（同一 helper）
	userID, meetingID, ok := parseIDs(c)
	if !ok {
		return
	}
	st, err := uc.Execute(c.Request.Context(), userID, meetingID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, toStateResponse(st))
}
```

`parseIDs`：複製 `Retry` handler 中取 userID（ctxutil）與 `uuid.Parse(c.Param("id"))` 失敗回 `response.Fail(c, apperr.New(errcode.Param, "id"))` 的程式碼；若 meeting handler 已有可 export 的 helper 則改為共用。

- [ ] **Step 4: 路由** — `server.go` 於 `/meetings/:id/retry` 旁：

```go
		authed.GET("/meetings/:id/correction", deps.CorrectionHandler.Get)
		authed.POST("/meetings/:id/correction", deps.CorrectionHandler.Start)
		authed.POST("/meetings/:id/correction/revert", deps.CorrectionHandler.Revert)
```

deps 加 `CorrectionHandler *correctionhandler.Handler`；main 以 `appcorrection.NewGetUC/NewStartUC/NewRevertUC` 注入。

- [ ] **Step 5: 全測 + commit** — `go vet ./... && go test ./...`；`git commit -m "feat(correction): HTTP endpoints 與 OpenAPI"`

---

### Task 11: 前端

**Files:**
- Modify: `busy-bee-fe/src/services/api/schema.d.ts`（`npm run gen:api`）、`client.ts`
- Create: `busy-bee-fe/src/components/CorrectionPanel.tsx`、`CorrectionPanel.test.tsx`
- Modify: `busy-bee-fe/src/components/TranscriptEditor.tsx`、新增 `TranscriptEditor.test.tsx`
- Modify: `busy-bee-fe/src/pages/MeetingDetailPage.tsx`、`hooks/useMeetingDetail.ts`

**Interfaces:**
- Consumes: Task 10 API
- Produces: `getCorrection / startCorrection / revertCorrection(idToken, meetingId): Promise<{ correction: CorrectionState }>`；`<CorrectionPanel meeting={MeetingDetail} onChanged={() => void} />`

- [ ] **Step 1: 產生 schema 與 client**

```bash
cd busy-bee-fe && npm run gen:api
```

`client.ts`：

```ts
export type CorrectionState = components['schemas']['CorrectionState']

export function getCorrection(idToken: string, meetingId: string): Promise<{ correction: CorrectionState }> {
  return request<{ correction: CorrectionState }>(`/api/v1/meetings/${meetingId}/correction`, {}, idToken)
}
export function startCorrection(idToken: string, meetingId: string): Promise<{ correction: CorrectionState }> {
  return request<{ correction: CorrectionState }>(`/api/v1/meetings/${meetingId}/correction`, { method: 'POST' }, idToken)
}
export function revertCorrection(idToken: string, meetingId: string): Promise<{ correction: CorrectionState }> {
  return request<{ correction: CorrectionState }>(`/api/v1/meetings/${meetingId}/correction/revert`, { method: 'POST' }, idToken)
}
```

- [ ] **Step 2: CorrectionPanel 失敗測試**（沿用 `SummarySections.test.tsx` 的 Testing Library 寫法；mock `useApiCall` 的 `call` 回傳）

```tsx
describe('CorrectionPanel', () => {
  it('低信心 ≥5% 顯示建議提示', async () => { /* uncertainRatio=0.08 → 出現「約 8% 內容語音辨識不確定，建議校正」 */ })
  it('低於門檻不顯示提示但仍有按鈕', async () => { /* 0.02 → 無提示、有「AI 校正」按鈕 */ })
  it('已用 2 次時按鈕停用並顯示 2/2', async () => { /* attemptsUsed=2, status=failed */ })
  it('進行中顯示校正中且按鈕停用', async () => { /* status=running */ })
  it('completed 時顯示還原按鈕', async () => { /* canRevert=true */ })
  it('按下校正先顯示確認文案（含失敗也計次）', async () => { /* 點按鈕 → 出現「每場最多 2 次，失敗也會計次」 */ })
})
```

每個 `/* */` 寫成完整 `render` + `screen` 斷言。

- [ ] **Step 3: 實作 `CorrectionPanel.tsx`**

```tsx
const SUGGEST_THRESHOLD = 0.05

/** AI 錄音重新校正：按鈕、次數、建議提示、進行中/失敗狀態、整份還原。 */
export function CorrectionPanel({ meeting, onChanged }: { meeting: MeetingDetail; onChanged: () => void }) {
  const { call } = useApiCall()
  const [state, setState] = useState<CorrectionState | null>(null)
  const [confirming, setConfirming] = useState(false)
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    const r = await call(getCorrection, meeting.id)
    setState(r.correction)
  }, [call, meeting.id])

  useEffect(() => { void load() }, [load, meeting.updatedAt])

  if (!state || meeting.status !== 'completed') return null
  const running = state.status === 'queued' || state.status === 'running'
  const exhausted = state.attemptsUsed >= state.attemptsMax
  const pct = Math.round(meeting.uncertainRatio * 100)

  const start = async () => {
    setBusy(true)
    try {
      setState((await call(startCorrection, meeting.id)).correction)
      setConfirming(false)
    } finally {
      setBusy(false)
    }
  }
  const revert = async () => {
    setBusy(true)
    try {
      setState((await call(revertCorrection, meeting.id)).correction)
      onChanged()
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="flex flex-col gap-2 rounded-lg border border-border bg-surface px-3 py-3 text-sm">
      {meeting.uncertainRatio >= SUGGEST_THRESHOLD && !running && (
        <p className="m-0 text-xs text-muted">本紀錄約 {pct}% 內容語音辨識不確定，建議校正。</p>
      )}
      <div className="flex items-center justify-between gap-2">
        <span className="text-xs text-muted">
          AI 校正 {state.attemptsUsed}/{state.attemptsMax}
          {running && '・校正中…'}
          {state.status === 'failed' && `・${state.errorMessage || '校正失敗'}`}
        </span>
        <div className="flex gap-2">
          {state.canRevert && (
            <button type="button" className="btn btn-secondary h-9" disabled={busy} onClick={() => void revert()}>
              還原
            </button>
          )}
          <button type="button" className="btn btn-primary h-9" disabled={busy || running || exhausted}
            onClick={() => setConfirming(true)}>
            AI 校正
          </button>
        </div>
      </div>
      {confirming && (
        <Sheet onClose={() => setConfirming(false)} title="AI 重新聽錄音校正">
          <p className="text-sm leading-6">
            AI 會重新聽錄音修正逐字稿並更新摘要。每場最多 2 次，失敗也會計次；你手動修改過的段落與行動項不會被覆蓋，完成後可一鍵還原。
            新版之前手動修改過的段落無法辨識，可能會被校正。
          </p>
          <button type="button" className="btn btn-primary mt-3 h-10 w-full" disabled={busy} onClick={() => void start()}>
            開始校正
          </button>
        </Sheet>
      )}
    </section>
  )
}
```

（`Sheet` 的 props 以 `components/Sheet.tsx` 實際介面為準調整；`meeting.updatedAt` 若不存在於 schema，改用 WS 事件觸發的 `onResync` 重新 load。）

- [ ] **Step 4: 掛到頁面** — `MeetingDetailPage.tsx` 在 `SummarySections` 上方渲染 `<CorrectionPanel meeting={meeting} onChanged={reload} />`；`useMeetingDetail` 回傳物件加 `reload: load`（既有 `load` 函式），WS 收到同 meeting 事件時已會 resync（確認 `correctionStatus` 事件也觸發 resync：`useMeetingStatusSocket` 的 handler 以 meetingId 比對即可，無需判斷 status）。

- [ ] **Step 5: TranscriptEditor「AI 已校正」** — 失敗測試 `TranscriptEditor.test.tsx`：

```tsx
it('有 originalText 的段落顯示 AI 已校正，點擊顯示原文', async () => {
  render(<TranscriptEditor meeting={meetingWith([{ speaker: 'A', text: '小王處理', originalText: '小娃處理', startMs: 0, endMs: 1000, manuallyEdited: false }])} onUpdated={() => {}} />)
  await userEvent.click(screen.getByRole('button', { name: 'AI 已校正' }))
  expect(screen.getByText('小娃處理')).toBeInTheDocument()
})
```

實作：`SegmentRow` 標頭列在時間/講者後，`seg.originalText` 有值時加：

```tsx
<button type="button" className="rounded-full bg-accent/10 px-2 py-0.5 text-[11px] text-accent"
  onClick={() => setShowOriginal(true)}>
  AI 已校正
</button>
```

並以 `Sheet` 顯示「校正前原文」+ `seg.originalText`。

- [ ] **Step 6: 前端驗證 + commit**

```bash
cd busy-bee-fe && npm run typecheck && npm run lint && npm run test:run && npm run build
git add -A && git commit -m "feat(fe): AI 校正面板與段落校正標籤"
```

---

### Task 12: 文件與端到端驗收

**Files:**
- Modify: `docs/PRODUCT.md`（新增 F-CORRECT 與驗收條件）、`docs/PLAN.md`（Phase 27 任務清單、當前焦點、Session Log）、`docs/ARCHITECTURE.md`（ADR-016：校正流程、tags 不動、切段參數、行動項規則、舊資料限制）、`CLAUDE.md`（「判斷要動哪個檔案」加一列：校正 prompt 在 `prompts/correct_transcript.md`、流程在 `application/correction/`）

- [ ] **Step 1: 文件更新**（依上列；PLAN 的 Phase 27 每個 Task 對應一項並填 commit hash）

- [ ] **Step 2: 本地 e2e**
  1. `docker compose up -d`、套 migration 000017、`go run ./cmd/server`、`npm run dev`。
  2. 上傳 Phase 26 的 `zh_noisy.wav`（scratchpad，或以 `say` + ffmpeg 重新產生）→ 完成後按「AI 校正」→ 確認：狀態 queued→running→completed 即時更新、段落出現「AI 已校正」且原文正確、摘要更新、2/2 計次。
  3. 校正中手動編輯一段 → 完成後該段維持人工版本。
  4. 手動新增一個行動項、勾選一個 AI 項 → 校正後兩者仍在、無重複。
  5. 還原 → 逐字稿/摘要回到校正前、行動項不變；第 3 次按鈕停用。
  6. 一場 ≥ 40 分鐘真實會議：確認切段數（log `chunks`）、段落銜接處無斷裂、總耗時與 Gemini usage（log）符合 ~US$0.05–0.07/小時。

- [ ] **Step 3: 全測 + commit**

```bash
cd busy-bee-be && go vet ./... && go test ./...
cd ../busy-bee-fe && npm run typecheck && npm run lint && npm run test:run && npm run build
git add -A && git commit -m "docs: Phase 27 AI 錄音重新校正（ADR-016、PRODUCT、PLAN）"
```
