package db

import (
	"context"
	"testing"

	domainartifact "github.com/as130232/busy-bee/busy-bee-be/domain/artifact"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

func TestArtifactRepo_UpsertAndList(t *testing.T) {
	pool := testPool(t)
	u := testUser(t, pool)
	m, err := NewMeetingRepo(pool).Create(context.Background(), domainmeeting.Meeting{
		UserID: u.ID, Title: "m", Status: domainmeeting.StatusAnalyzing,
	})
	if err != nil {
		t.Fatal(err)
	}
	repo := NewArtifactRepo(pool)

	first, err := repo.Upsert(context.Background(), m.ID, domainartifact.TypePRD, "# PRD v1")
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if first.Type != domainartifact.TypePRD || first.Content != "# PRD v1" {
		t.Errorf("first = %+v", first)
	}

	// 同 meeting 同 type 重寫 → 覆蓋而非新增（冪等）
	second, err := repo.Upsert(context.Background(), m.ID, domainartifact.TypePRD, "# PRD v2")
	if err != nil {
		t.Fatalf("re-upsert error = %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("re-upsert created new row: %v != %v", second.ID, first.ID)
	}

	if _, err := repo.Upsert(context.Background(), m.ID, domainartifact.TypeTechSpec, "# Spec"); err != nil {
		t.Fatal(err)
	}

	list, err := repo.ListByMeeting(context.Background(), m.ID)
	if err != nil {
		t.Fatalf("ListByMeeting() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
	if list[0].Content != "# PRD v2" {
		t.Errorf("prd content = %q, want overwritten v2", list[0].Content)
	}
}
