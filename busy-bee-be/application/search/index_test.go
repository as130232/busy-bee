package search

import (
	"context"
	"testing"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainsearch "github.com/as130232/busy-bee/busy-bee-be/domain/search"
)

type fakeMeetingRepo struct{ m domainmeeting.Meeting }

func (f *fakeMeetingRepo) Get(_ context.Context, _ uuid.UUID) (domainmeeting.Meeting, error) {
	return f.m, nil
}

type fakeEmbedder struct{ calls int }

func (f *fakeEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	f.calls++
	return []float32{0.1, 0.2}, nil
}

type fakeChunkRepo struct {
	upserted []domainsearch.Chunk
}

func (f *fakeChunkRepo) Upsert(_ context.Context, cs []domainsearch.Chunk) error {
	f.upserted = cs
	return nil
}
func (f *fakeChunkRepo) DeleteByMeeting(context.Context, uuid.UUID) error { return nil }
func (f *fakeChunkRepo) SearchSimilar(context.Context, uuid.UUID, []float32, int) ([]domainsearch.SearchResult, error) {
	return nil, nil
}
func (f *fakeChunkRepo) MeetingsWithoutChunks(context.Context) ([]uuid.UUID, error) {
	return nil, nil
}

func TestIndexUC_ChunksEmbedAndUpsert(t *testing.T) {
	mid := uuid.New()
	uid := uuid.New()
	mrepo := &fakeMeetingRepo{m: domainmeeting.Meeting{ID: mid, UserID: uid,
		Status: domainmeeting.StatusCompleted, Transcript: "第一句很長的內容。第二句也很長的內容。"}}
	emb := &fakeEmbedder{}
	crepo := &fakeChunkRepo{}
	uc := NewIndexUC(mrepo, emb, crepo)

	if err := uc.Execute(context.Background(), mid); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(crepo.upserted) == 0 {
		t.Fatal("expected chunks upserted")
	}
	if emb.calls != len(crepo.upserted) {
		t.Errorf("embed calls %d != chunks %d", emb.calls, len(crepo.upserted))
	}
	if crepo.upserted[0].UserID != uid || crepo.upserted[0].MeetingID != mid {
		t.Error("chunk missing owner/meeting id")
	}
}

func TestIndexUC_EmptyTranscriptSkips(t *testing.T) {
	mid := uuid.New()
	mrepo := &fakeMeetingRepo{m: domainmeeting.Meeting{ID: mid, Status: domainmeeting.StatusCompleted, Transcript: ""}}
	emb := &fakeEmbedder{}
	crepo := &fakeChunkRepo{}
	uc := NewIndexUC(mrepo, emb, crepo)

	if err := uc.Execute(context.Background(), mid); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if emb.calls != 0 || len(crepo.upserted) != 0 {
		t.Error("empty transcript should skip embedding/upsert")
	}
}
