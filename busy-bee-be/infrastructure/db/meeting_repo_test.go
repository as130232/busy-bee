package db

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
)

func testUser(t *testing.T, pool *pgxpool.Pool) domainuser.User {
	t.Helper()
	uid := fmt.Sprintf("meeting-test-%d", time.Now().UnixNano())
	u, err := NewUserRepo(pool).UpsertByFirebaseUID(context.Background(),
		domainuser.Identity{UID: uid, Email: uid + "@test.com"})
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM meetings WHERE user_id = $1", u.ID)
		pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", u.ID)
	})
	return u
}

func TestMeetingRepo_CreateAndGet(t *testing.T) {
	pool := testPool(t)
	u := testUser(t, pool)
	repo := NewMeetingRepo(pool)

	created, err := repo.Create(context.Background(), domainmeeting.Meeting{
		UserID: u.ID,
		Title:  "架構討論",
		Status: domainmeeting.StatusPending,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == uuid.Nil || created.Status != domainmeeting.StatusPending {
		t.Errorf("created = %+v, want ID set and pending status", created)
	}
	if created.RemindBeforeMin != 15 {
		t.Errorf("RemindBeforeMin = %d, want default 15", created.RemindBeforeMin)
	}

	got, err := repo.GetForUser(context.Background(), created.ID, u.ID)
	if err != nil {
		t.Fatalf("GetForUser() error = %v", err)
	}
	if got.Title != "架構討論" {
		t.Errorf("Title = %q, want 架構討論", got.Title)
	}
}

func TestMeetingRepo_GetForUser_OtherUserGetsNotFound(t *testing.T) {
	pool := testPool(t)
	u := testUser(t, pool)
	repo := NewMeetingRepo(pool)

	created, err := repo.Create(context.Background(), domainmeeting.Meeting{
		UserID: u.ID, Title: "私人會議", Status: domainmeeting.StatusPending,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = repo.GetForUser(context.Background(), created.ID, uuid.New()) // 別人的 userID
	if !errors.Is(err, domainmeeting.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound (owner-only visibility)", err)
	}
}

func TestMeetingRepo_UpdateStatus_OptimisticCheck(t *testing.T) {
	pool := testPool(t)
	u := testUser(t, pool)
	repo := NewMeetingRepo(pool)

	created, err := repo.Create(context.Background(), domainmeeting.Meeting{
		UserID: u.ID, Title: "m", Status: domainmeeting.StatusPending,
	})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := repo.UpdateStatus(context.Background(), created.ID,
		domainmeeting.StatusPending, domainmeeting.StatusTranscribing)
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if updated.Status != domainmeeting.StatusTranscribing {
		t.Errorf("Status = %s, want transcribing", updated.Status)
	}

	// from-status 不符時應失敗（防止併發重複轉移）
	_, err = repo.UpdateStatus(context.Background(), created.ID,
		domainmeeting.StatusPending, domainmeeting.StatusTranscribing)
	if !errors.Is(err, domainmeeting.ErrStatusConflict) {
		t.Fatalf("err = %v, want ErrStatusConflict", err)
	}
}

func TestMeetingRepo_ListForUser_SearchAndOwnerFilter(t *testing.T) {
	pool := testPool(t)
	u := testUser(t, pool)
	other := testUser(t, pool)
	repo := NewMeetingRepo(pool)
	ctx := context.Background()

	m1, _ := repo.Create(ctx, domainmeeting.Meeting{UserID: u.ID, Title: "架構評審會議", Status: domainmeeting.StatusCompleted})
	repo.SaveTranscript(ctx, m1.ID, "今天討論了 pgvector 的導入", 60)
	repo.Create(ctx, domainmeeting.Meeting{UserID: u.ID, Title: "每週例會", Status: domainmeeting.StatusPending})
	repo.Create(ctx, domainmeeting.Meeting{UserID: other.ID, Title: "別人的架構會議", Status: domainmeeting.StatusPending})

	all, err := repo.ListForUser(ctx, u.ID, "")
	if err != nil {
		t.Fatalf("ListForUser() error = %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("len = %d, want 2 (owner-only)", len(all))
	}
	if all[0].Title != "每週例會" {
		t.Errorf("order wrong: first = %q, want newest first", all[0].Title)
	}

	byTitle, _ := repo.ListForUser(ctx, u.ID, "架構")
	if len(byTitle) != 1 || byTitle[0].Title != "架構評審會議" {
		t.Errorf("search by title = %+v, want only 架構評審會議 (not other's)", byTitle)
	}

	byTranscript, _ := repo.ListForUser(ctx, u.ID, "pgvector")
	if len(byTranscript) != 1 {
		t.Errorf("search by transcript len = %d, want 1", len(byTranscript))
	}

	none, _ := repo.ListForUser(ctx, u.ID, "不存在的關鍵字xyz")
	if len(none) != 0 {
		t.Errorf("no-match search len = %d, want 0", len(none))
	}
}
