//go:build integration

package db

import (
	"context"
	"errors"
	"testing"
	"time"

	domainmc "github.com/as130232/busy-bee/busy-bee-be/domain/meetingcontext"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

func TestLiveRepo_Lifecycle(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	u := testUser(t, pool)
	repo := NewMeetingRepo(pool)
	before := time.Now().Add(-time.Minute)

	m, err := repo.CreateLive(ctx, domainmeeting.Meeting{UserID: u.ID, Title: "即時", AudioGCSPath: "audio/x.webm"})
	if err != nil || m.Status != domainmeeting.StatusLive || m.LiveStartedAt == nil || m.LiveNotes != nil {
		t.Fatalf("CreateLive = %+v err %v", m, err)
	}
	if _, err := repo.AddLiveSeconds(ctx, m.ID, u.ID, 90); err != nil {
		t.Fatalf("AddLiveSeconds: %v", err)
	}
	if m, err = repo.AddLiveSeconds(ctx, m.ID, u.ID, 30); err != nil || m.LiveSeconds != 120 {
		t.Fatalf("AddLiveSeconds total = %d err %v", m.LiveSeconds, err)
	}
	if total, err := repo.SumLiveSecondsSince(ctx, u.ID, before); err != nil || total != 120 {
		t.Fatalf("SumLiveSecondsSince = %d err %v", total, err)
	}
	if total, _ := repo.SumLiveSecondsSince(ctx, u.ID, time.Now().Add(time.Hour)); total != 0 {
		t.Fatalf("future since should be 0, got %d", total)
	}

	notes := domainmeeting.LiveNotes{Notes: domainmeeting.LiveSummary{Current: "討論部署", Points: []string{"週三上線"}}}
	if m, err = repo.FinishLive(ctx, m.ID, u.ID, notes); err != nil || m.Status != domainmeeting.StatusScheduled {
		t.Fatalf("FinishLive = %+v err %v", m, err)
	}
	if m.LiveNotes == nil || m.LiveNotes.Notes.Current != "討論部署" || m.LiveNotes.QA == nil {
		t.Fatalf("live notes roundtrip = %+v", m.LiveNotes)
	}
	if _, err := repo.FinishLive(ctx, m.ID, u.ID, notes); !errors.Is(err, domainmeeting.ErrStatusConflict) {
		t.Fatalf("finish twice want ErrStatusConflict, got %v", err)
	}
	if _, err := repo.AddLiveSeconds(ctx, m.ID, u.ID, 10); !errors.Is(err, domainmeeting.ErrStatusConflict) {
		t.Fatalf("heartbeat after finish want ErrStatusConflict, got %v", err)
	}
}

func TestLiveRepo_StartFromScheduled(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	u, other := testUser(t, pool), testUser(t, pool)
	repo := NewMeetingRepo(pool)
	at := time.Now().Add(time.Hour)
	s, err := repo.Create(ctx, domainmeeting.Meeting{UserID: u.ID, Title: "排程", Status: domainmeeting.StatusScheduled, ScheduledAt: &at})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.StartLiveFromScheduled(ctx, s.ID, other.ID, "p"); !errors.Is(err, domainmeeting.ErrStatusConflict) {
		t.Fatalf("other user want conflict, got %v", err)
	}
	m, err := repo.StartLiveFromScheduled(ctx, s.ID, u.ID, "audio/y.webm")
	if err != nil || m.Status != domainmeeting.StatusLive || m.AudioGCSPath != "audio/y.webm" || m.LiveStartedAt == nil {
		t.Fatalf("StartLiveFromScheduled = %+v err %v", m, err)
	}
}

func TestMeetingContextRepo(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	u, other := testUser(t, pool), testUser(t, pool)
	m, err := NewMeetingRepo(pool).CreateLive(ctx, domainmeeting.Meeting{UserID: u.ID, Title: "即時", AudioGCSPath: "a"})
	if err != nil {
		t.Fatal(err)
	}
	repo := NewMeetingContextRepo(pool)
	c, err := repo.Create(ctx, domainmc.Context{MeetingID: m.ID, UserID: u.ID, Title: "規格", Source: domainmc.SourceText, Content: "中文內容"})
	if err != nil || c.Keyterms == nil {
		t.Fatalf("Create = %+v err %v", c, err)
	}
	if n, _ := repo.SumChars(ctx, m.ID, u.ID); n != 4 {
		t.Errorf("SumChars = %d, want 4 (chars, not bytes)", n)
	}
	if list, _ := repo.List(ctx, m.ID, other.ID); len(list) != 0 {
		t.Errorf("other user sees %d contexts", len(list))
	}
	if _, err := repo.Delete(ctx, c.ID, m.ID, other.ID); !errors.Is(err, domainmc.ErrNotFound) {
		t.Errorf("other user delete want ErrNotFound, got %v", err)
	}
	if _, err := repo.Delete(ctx, c.ID, m.ID, u.ID); err != nil {
		t.Errorf("delete: %v", err)
	}
	if list, _ := repo.List(ctx, m.ID, u.ID); len(list) != 0 {
		t.Errorf("after delete got %d", len(list))
	}
}
