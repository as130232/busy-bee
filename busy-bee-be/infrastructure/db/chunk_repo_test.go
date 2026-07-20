package db

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/domain/search"
)

func mkVec(seed float32) []float32 {
	v := make([]float32, 768)
	for i := range v {
		v[i] = seed
	}
	return v
}

// completedMeeting 建一筆 completed 且有 transcript 的會議（Create 不寫 transcript，故直接 UPDATE）。
func completedMeeting(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, title, transcript string) domainmeeting.Meeting {
	t.Helper()
	m, err := NewMeetingRepo(pool).Create(context.Background(), domainmeeting.Meeting{
		UserID: userID, Title: title, Status: domainmeeting.StatusPending,
	})
	if err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`UPDATE meetings SET status = 'completed', transcript = $1 WHERE id = $2`, transcript, m.ID); err != nil {
		t.Fatalf("set completed transcript: %v", err)
	}
	return m
}

func TestChunkRepo_UpsertAndSearchSimilar(t *testing.T) {
	pool := testPool(t)
	u := testUser(t, pool)
	m := completedMeeting(t, pool, u.ID, "定價會議", "談價格策略")
	repo := NewChunkRepo(pool)

	err := repo.Upsert(context.Background(), []search.Chunk{
		{MeetingID: m.ID, UserID: u.ID, ChunkIndex: 0, Content: "談價格策略", Embedding: mkVec(0.1)},
		{MeetingID: m.ID, UserID: u.ID, ChunkIndex: 1, Content: "無關內容", Embedding: mkVec(0.9)},
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	res, err := repo.SearchSimilar(context.Background(), u.ID, mkVec(0.1), 5)
	if err != nil {
		t.Fatalf("SearchSimilar() error = %v", err)
	}
	if len(res) == 0 || res[0].MeetingID != m.ID {
		t.Fatalf("expected meeting %s in results, got %#v", m.ID, res)
	}
	if res[0].Snippet != "談價格策略" {
		t.Errorf("snippet = %q, want 談價格策略", res[0].Snippet)
	}
	if res[0].MatchType != search.MatchSemantic {
		t.Errorf("matchType = %q, want %q", res[0].MatchType, search.MatchSemantic)
	}

	// 非本人搜不到
	other, _ := repo.SearchSimilar(context.Background(), uuid.New(), mkVec(0.1), 5)
	if len(other) != 0 {
		t.Errorf("other user should get no results, got %d", len(other))
	}
}

func TestChunkRepo_UpsertReplacesExisting(t *testing.T) {
	pool := testPool(t)
	u := testUser(t, pool)
	m := completedMeeting(t, pool, u.ID, "m", "x")
	repo := NewChunkRepo(pool)
	_ = repo.Upsert(context.Background(), []search.Chunk{{MeetingID: m.ID, UserID: u.ID, ChunkIndex: 0, Content: "a", Embedding: mkVec(0.1)}})
	// 重跑（冪等 upsert：先刪該會議舊 chunks 再插）
	if err := repo.Upsert(context.Background(), []search.Chunk{{MeetingID: m.ID, UserID: u.ID, ChunkIndex: 0, Content: "b", Embedding: mkVec(0.2)}}); err != nil {
		t.Fatalf("re-Upsert error = %v", err)
	}
	res, _ := repo.SearchSimilar(context.Background(), u.ID, mkVec(0.2), 5)
	if len(res) != 1 || res[0].Snippet != "b" {
		t.Errorf("expected single replaced chunk 'b', got %#v", res)
	}
}

func TestChunkRepo_DeleteByMeeting(t *testing.T) {
	pool := testPool(t)
	u := testUser(t, pool)
	m := completedMeeting(t, pool, u.ID, "m", "x")
	repo := NewChunkRepo(pool)
	_ = repo.Upsert(context.Background(), []search.Chunk{{MeetingID: m.ID, UserID: u.ID, ChunkIndex: 0, Content: "a", Embedding: mkVec(0.1)}})
	if err := repo.DeleteByMeeting(context.Background(), m.ID); err != nil {
		t.Fatalf("DeleteByMeeting() error = %v", err)
	}
	res, _ := repo.SearchSimilar(context.Background(), u.ID, mkVec(0.1), 5)
	if len(res) != 0 {
		t.Errorf("expected no chunks after delete, got %#v", res)
	}
}

func TestChunkRepo_MeetingsWithoutChunks(t *testing.T) {
	pool := testPool(t)
	u := testUser(t, pool)
	indexed := completedMeeting(t, pool, u.ID, "i", "x")
	missing := completedMeeting(t, pool, u.ID, "m", "y")
	repo := NewChunkRepo(pool)
	_ = repo.Upsert(context.Background(), []search.Chunk{{MeetingID: indexed.ID, UserID: u.ID, ChunkIndex: 0, Content: "x", Embedding: mkVec(0.1)}})

	ids, err := repo.MeetingsWithoutChunks(context.Background())
	if err != nil {
		t.Fatalf("MeetingsWithoutChunks() error = %v", err)
	}
	found := false
	for _, id := range ids {
		if id == missing.ID {
			found = true
		}
		if id == indexed.ID {
			t.Error("indexed meeting should not appear")
		}
	}
	if !found {
		t.Errorf("missing meeting %s should appear in results", missing.ID)
	}
}
