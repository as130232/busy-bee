package worker

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

type fakeChunkScanner struct{ ids []uuid.UUID }

func (f *fakeChunkScanner) MeetingsWithoutChunks(context.Context) ([]uuid.UUID, error) {
	return f.ids, nil
}

type recordingIndexer struct{ indexed []uuid.UUID }

func (r *recordingIndexer) Execute(_ context.Context, id uuid.UUID) error {
	r.indexed = append(r.indexed, id)
	return nil
}

func TestBackfillIndexesMissingMeetings(t *testing.T) {
	want := uuid.New()
	scanner := &fakeChunkScanner{ids: []uuid.UUID{want}}
	idx := &recordingIndexer{}

	backfillOnce(context.Background(), scanner, idx)

	if len(idx.indexed) != 1 || idx.indexed[0] != want {
		t.Fatalf("expected backfill to index %s, got %#v", want, idx.indexed)
	}
}
