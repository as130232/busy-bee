package search

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainsearch "github.com/as130232/busy-bee/busy-bee-be/domain/search"
)

type fakeLiteral struct {
	meetings []domainmeeting.Meeting
	err      error
}

func (f *fakeLiteral) ListForUser(_ context.Context, _ uuid.UUID, _ string) ([]domainmeeting.Meeting, error) {
	return f.meetings, f.err
}

// fakeOwner 依 id 造出一筆屬於該 user 的會議（模擬 GetForUser owner 過濾）。
type fakeOwner struct{ err error }

func (f *fakeOwner) GetForUser(_ context.Context, id, _ uuid.UUID) (domainmeeting.Meeting, error) {
	if f.err != nil {
		return domainmeeting.Meeting{}, f.err
	}
	return domainmeeting.Meeting{ID: id, Title: "語意會議"}, nil
}

type searchFakeChunks struct {
	results []domainsearch.SearchResult
}

func (f *searchFakeChunks) Upsert(context.Context, []domainsearch.Chunk) error { return nil }
func (f *searchFakeChunks) DeleteByMeeting(context.Context, uuid.UUID) error   { return nil }
func (f *searchFakeChunks) SearchSimilar(context.Context, uuid.UUID, []float32, int) ([]domainsearch.SearchResult, error) {
	return f.results, nil
}
func (f *searchFakeChunks) MeetingsWithoutChunks(context.Context) ([]uuid.UUID, error) {
	return nil, nil
}

type failEmbedder struct{}

func (f *failEmbedder) Embed(context.Context, string) ([]float32, error) {
	return nil, errors.New("embed down")
}

func TestSearchUC_MergesSemanticAndLiteral(t *testing.T) {
	semID := uuid.New()
	litID := uuid.New()
	lit := &fakeLiteral{meetings: []domainmeeting.Meeting{{ID: litID, Title: "字面命中"}}}
	chunks := &searchFakeChunks{results: []domainsearch.SearchResult{{MeetingID: semID, Snippet: "語意片段", Score: 0.9, MatchType: domainsearch.MatchSemantic}}}
	uc := NewSearchUC(lit, &fakeEmbedder{}, chunks, &fakeOwner{})

	meetings, hits, err := uc.Execute(context.Background(), uuid.New(), "查詢")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	ids := map[uuid.UUID]bool{}
	for _, m := range meetings {
		ids[m.ID] = true
	}
	if !ids[semID] || !ids[litID] {
		t.Errorf("expected both semantic %s and literal %s, got %#v", semID, litID, ids)
	}
	if hits[semID].Snippet != "語意片段" {
		t.Errorf("semantic hit snippet missing: %#v", hits[semID])
	}
}

func TestSearchUC_EmbedFailsFallsBackToLiteral(t *testing.T) {
	litID := uuid.New()
	lit := &fakeLiteral{meetings: []domainmeeting.Meeting{{ID: litID, Title: "字面"}}}
	uc := NewSearchUC(lit, &failEmbedder{}, &searchFakeChunks{}, &fakeOwner{})

	meetings, _, err := uc.Execute(context.Background(), uuid.New(), "查詢")
	if err != nil {
		t.Fatalf("Execute() should not error on embed failure, got %v", err)
	}
	if len(meetings) != 1 || meetings[0].ID != litID {
		t.Errorf("expected literal fallback result, got %#v", meetings)
	}
}
