package search

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainsearch "github.com/as130232/busy-bee/busy-bee-be/domain/search"
)

// longDistinctTranscript 產生一段夠長、句句不同的逐字稿，確保 SplitIntoChunks 切成多塊。
func longDistinctTranscript(sentences int) string {
	var b strings.Builder
	for i := 0; i < sentences; i++ {
		fmt.Fprintf(&b, "第%d段內容講述不同的主題與細節以確保切塊。", i)
	}
	return b.String()
}

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
	upserted     []domainsearch.Chunk
	existing     map[string][]float32
	deletedID    uuid.UUID
	deleteCalled bool
}

func (f *fakeChunkRepo) Upsert(_ context.Context, cs []domainsearch.Chunk) error {
	f.upserted = cs
	return nil
}
func (f *fakeChunkRepo) DeleteByMeeting(_ context.Context, id uuid.UUID) error {
	f.deleteCalled = true
	f.deletedID = id
	return nil
}
func (f *fakeChunkRepo) SearchSimilar(context.Context, uuid.UUID, []float32, int) ([]domainsearch.SearchResult, error) {
	return nil, nil
}
func (f *fakeChunkRepo) MeetingsWithoutChunks(context.Context) ([]uuid.UUID, error) {
	return nil, nil
}
func (f *fakeChunkRepo) ExistingEmbeddings(context.Context, uuid.UUID) (map[string][]float32, error) {
	return f.existing, nil
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

func TestIndexUC_EmptyTranscriptDeletesStaleChunks(t *testing.T) {
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
	// 空逐字稿應清掉殘留舊 chunks（避免搜到已不存在的內容）
	if !crepo.deleteCalled || crepo.deletedID != mid {
		t.Errorf("empty transcript should DeleteByMeeting(%s), deleteCalled=%v id=%s", mid, crepo.deleteCalled, crepo.deletedID)
	}
}

// P4：內容未變動的片段應複用既有 embedding，不重複呼叫 embed（省成本）。
func TestIndexUC_ReusesUnchangedEmbeddings(t *testing.T) {
	mid := uuid.New()
	uid := uuid.New()
	transcript := longDistinctTranscript(40)
	parts := domainsearch.SplitIntoChunks(transcript, chunkTargetChars, chunkOverlap)
	if len(parts) < 2 {
		t.Fatalf("test setup: transcript should chunk into >=2, got %d", len(parts))
	}
	// 既有索引已涵蓋全部片段內容 → 一次 embed 都不該發生
	existing := map[string][]float32{}
	for _, p := range parts {
		existing[p] = []float32{0.42}
	}
	mrepo := &fakeMeetingRepo{m: domainmeeting.Meeting{ID: mid, UserID: uid,
		Status: domainmeeting.StatusCompleted, Transcript: transcript}}
	emb := &fakeEmbedder{}
	crepo := &fakeChunkRepo{existing: existing}
	uc := NewIndexUC(mrepo, emb, crepo)

	if err := uc.Execute(context.Background(), mid); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if emb.calls != 0 {
		t.Errorf("unchanged chunks should reuse embeddings, embed called %d times", emb.calls)
	}
	if len(crepo.upserted) != len(parts) {
		t.Fatalf("upserted %d chunks, want %d", len(crepo.upserted), len(parts))
	}
	// 複用的 embedding 應被沿用
	if len(crepo.upserted[0].Embedding) != 1 || crepo.upserted[0].Embedding[0] != 0.42 {
		t.Errorf("reused embedding not carried, got %#v", crepo.upserted[0].Embedding)
	}
}

// 只有變動的片段需要重嵌，未變動的複用。
func TestIndexUC_EmbedsOnlyChangedChunks(t *testing.T) {
	mid := uuid.New()
	uid := uuid.New()
	transcript := longDistinctTranscript(40)
	parts := domainsearch.SplitIntoChunks(transcript, chunkTargetChars, chunkOverlap)
	if len(parts) < 2 {
		t.Fatalf("test setup: transcript should chunk into >=2, got %d", len(parts))
	}
	// 只提供除最後一塊外的既有 embedding → 只有最後一塊需要 embed
	existing := map[string][]float32{}
	for _, p := range parts[:len(parts)-1] {
		existing[p] = []float32{0.42}
	}
	mrepo := &fakeMeetingRepo{m: domainmeeting.Meeting{ID: mid, UserID: uid,
		Status: domainmeeting.StatusCompleted, Transcript: transcript}}
	emb := &fakeEmbedder{}
	crepo := &fakeChunkRepo{existing: existing}
	uc := NewIndexUC(mrepo, emb, crepo)

	if err := uc.Execute(context.Background(), mid); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if emb.calls != 1 {
		t.Errorf("only the changed chunk should embed, embed called %d times", emb.calls)
	}
}
