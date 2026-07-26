//go:build integration

package db

import (
	"context"
	"testing"

	"github.com/google/uuid"

	domainactionitem "github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	domainartifact "github.com/as130232/busy-bee/busy-bee-be/domain/artifact"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

const markerActionItems = domainartifact.Type("action_items")

// SaveExtraction 中途 insert 失敗時，整批（摘要 + 清空 + 插入）必須 rollback，
// 不得留下部分/空狀態。以不存在的 userID 觸發 action_items.user_id FK violation。
func TestProcessRepo_SaveExtraction_RollsBackOnInsertFailure(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	u := testUser(t, pool)
	meetings := NewMeetingRepo(pool)
	items := NewActionItemRepo(pool)
	proc := NewProcessRepo(pool)

	m, err := meetings.Create(ctx, domainmeeting.Meeting{
		UserID: u.ID, Title: "會議", Status: domainmeeting.StatusAnalyzing, Transcript: "t",
	})
	if err != nil {
		t.Fatalf("create meeting: %v", err)
	}

	// 第一次：成功落庫摘要 S1 + 2 筆行動項
	orig := []domainactionitem.Extracted{{Description: "A"}, {Description: "B"}}
	if err := proc.SaveExtraction(ctx, m.ID, u.ID, "S1", orig, markerActionItems, "[]"); err != nil {
		t.Fatalf("first SaveExtraction: %v", err)
	}
	if got, _ := items.ListByMeeting(ctx, m.ID); len(got) != 2 {
		t.Fatalf("seed items = %d, want 2", len(got))
	}

	// 第二次：userID 不存在 → Insert 觸發 FK 違反 → 應整批 rollback
	badUser := uuid.New()
	err = proc.SaveExtraction(ctx, m.ID, badUser, "S2-changed",
		[]domainactionitem.Extracted{{Description: "C"}}, markerActionItems, `["C"]`)
	if err == nil {
		t.Fatal("expected SaveExtraction to fail on FK violation")
	}

	// rollback 驗證：行動項仍為原本 2 筆（delete 已回滾）
	after, err := items.ListByMeeting(ctx, m.ID)
	if err != nil {
		t.Fatalf("list after rollback: %v", err)
	}
	if len(after) != 2 {
		t.Errorf("action_items after failed tx = %d, want 2 (delete must roll back)", len(after))
	}

	// rollback 驗證：摘要仍為 S1（SaveSummary 已回滾）
	got, err := meetings.Get(ctx, m.ID)
	if err != nil {
		t.Fatalf("get meeting: %v", err)
	}
	if got.Summary != "S1" {
		t.Errorf("summary after failed tx = %q, want S1 (SaveSummary must roll back)", got.Summary)
	}
}

// 正常路徑：摘要 + 行動項 + 標記都落庫。
func TestProcessRepo_SaveExtraction_Success(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	u := testUser(t, pool)
	meetings := NewMeetingRepo(pool)
	items := NewActionItemRepo(pool)
	arts := NewArtifactRepo(pool)
	proc := NewProcessRepo(pool)

	m, err := meetings.Create(ctx, domainmeeting.Meeting{
		UserID: u.ID, Title: "會議", Status: domainmeeting.StatusAnalyzing, Transcript: "t",
	})
	if err != nil {
		t.Fatalf("create meeting: %v", err)
	}

	if err := proc.SaveExtraction(ctx, m.ID, u.ID, "摘要",
		[]domainactionitem.Extracted{{Description: "做 A"}}, markerActionItems, `[{"description":"做 A"}]`); err != nil {
		t.Fatalf("SaveExtraction: %v", err)
	}

	if got, _ := items.ListByMeeting(ctx, m.ID); len(got) != 1 {
		t.Errorf("items = %d, want 1", len(got))
	}
	if got, _ := meetings.Get(ctx, m.ID); got.Summary != "摘要" {
		t.Errorf("summary = %q, want 摘要", got.Summary)
	}
	list, _ := arts.ListByMeeting(ctx, m.ID)
	var hasMarker bool
	for _, a := range list {
		if a.Type == markerActionItems {
			hasMarker = true
		}
	}
	if !hasMarker {
		t.Error("action_items marker not persisted")
	}
}
