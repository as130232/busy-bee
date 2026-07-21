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
	repo.SaveTranscript(ctx, m1.ID, "今天討論了 pgvector 的導入", nil, 60)
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

func TestMeetingRepo_RenameOwnerOnly(t *testing.T) {
	pool := testPool(t)
	u := testUser(t, pool)
	repo := NewMeetingRepo(pool)

	created, err := repo.Create(context.Background(), domainmeeting.Meeting{
		UserID: u.ID, Title: "舊名", Status: domainmeeting.StatusCompleted,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	renamed, err := repo.Rename(context.Background(), created.ID, u.ID, "新名")
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if renamed.Title != "新名" {
		t.Errorf("Title = %q, want 新名", renamed.Title)
	}

	// 非本人 → ErrNotFound
	if _, err := repo.Rename(context.Background(), created.ID, uuid.New(), "駭"); !errors.Is(err, domainmeeting.ErrNotFound) {
		t.Errorf("Rename by other user err = %v, want ErrNotFound", err)
	}
}

func TestMeetingRepo_DeleteScheduledOnly(t *testing.T) {
	pool := testPool(t)
	u := testUser(t, pool)
	repo := NewMeetingRepo(pool)

	at := time.Now().Add(2 * time.Hour)
	sched, err := repo.CreateScheduled(context.Background(), u.ID, domainmeeting.ScheduleParams{
		Title: "待刪行程", ScheduledAt: at, RemindBeforeMin: 15,
	})
	if err != nil {
		t.Fatalf("CreateScheduled() error = %v", err)
	}

	if err := repo.DeleteScheduled(context.Background(), sched.ID, u.ID); err != nil {
		t.Fatalf("DeleteScheduled() error = %v", err)
	}
	if _, err := repo.GetForUser(context.Background(), sched.ID, u.ID); !errors.Is(err, domainmeeting.ErrNotFound) {
		t.Errorf("after delete GetForUser err = %v, want ErrNotFound", err)
	}

	// 非 scheduled 狀態不可刪
	done, err := repo.Create(context.Background(), domainmeeting.Meeting{
		UserID: u.ID, Title: "已完成", Status: domainmeeting.StatusCompleted,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := repo.DeleteScheduled(context.Background(), done.ID, u.ID); !errors.Is(err, domainmeeting.ErrNotFound) {
		t.Errorf("delete non-scheduled err = %v, want ErrNotFound", err)
	}
}

func TestMeetingRepo_SaveTranscriptSegmentsRoundTrip(t *testing.T) {
	pool := testPool(t)
	u := testUser(t, pool)
	repo := NewMeetingRepo(pool)
	ctx := context.Background()

	m, err := repo.Create(ctx, domainmeeting.Meeting{
		UserID: u.ID, Title: "三人討論", Status: domainmeeting.StatusTranscribing,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	segs := []domainmeeting.TranscriptSegment{
		{Speaker: "A", Text: "我們先討論架構", StartMs: 0, EndMs: 1500},
		{Speaker: "B", Text: "用 Clean Architecture", StartMs: 1500, EndMs: 3200},
	}
	if _, err := repo.SaveTranscript(ctx, m.ID, "A: 我們先討論架構\nB: 用 Clean Architecture", segs, 3); err != nil {
		t.Fatalf("SaveTranscript() error = %v", err)
	}

	got, err := repo.GetForUser(ctx, m.ID, u.ID)
	if err != nil {
		t.Fatalf("GetForUser() error = %v", err)
	}
	if len(got.TranscriptSegments) != 2 {
		t.Fatalf("segments len = %d, want 2", len(got.TranscriptSegments))
	}
	if got.TranscriptSegments[0].Speaker != "A" || got.TranscriptSegments[0].Text != "我們先討論架構" ||
		got.TranscriptSegments[1].EndMs != 3200 {
		t.Errorf("segments round-trip mismatch: %+v", got.TranscriptSegments)
	}
}

func TestMeetingRepo_UpdateSpeakerNames(t *testing.T) {
	pool := testPool(t)
	u := testUser(t, pool)
	other := testUser(t, pool)
	repo := NewMeetingRepo(pool)
	ctx := context.Background()

	m, err := repo.Create(ctx, domainmeeting.Meeting{
		UserID: u.ID, Title: "命名測試", Status: domainmeeting.StatusCompleted,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	updated, err := repo.UpdateSpeakerNames(ctx, m.ID, u.ID, map[string]string{"A": "Ben"})
	if err != nil {
		t.Fatalf("UpdateSpeakerNames() error = %v", err)
	}
	if updated.SpeakerNames["A"] != "Ben" {
		t.Errorf("SpeakerNames = %v, want A=Ben", updated.SpeakerNames)
	}

	// round-trip 再讀一次
	got, err := repo.GetForUser(ctx, m.ID, u.ID)
	if err != nil {
		t.Fatalf("GetForUser() error = %v", err)
	}
	if got.SpeakerNames["A"] != "Ben" {
		t.Errorf("persisted SpeakerNames = %v, want A=Ben", got.SpeakerNames)
	}

	// 非本人不可改（owner 過濾）→ ErrNotFound
	if _, err := repo.UpdateSpeakerNames(ctx, m.ID, other.ID, map[string]string{"A": "駭客"}); !errors.Is(err, domainmeeting.ErrNotFound) {
		t.Errorf("non-owner UpdateSpeakerNames err = %v, want ErrNotFound", err)
	}
}
