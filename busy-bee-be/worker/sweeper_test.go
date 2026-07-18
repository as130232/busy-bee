package worker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeLister struct {
	ids [][]uuid.UUID // 每次呼叫回傳下一組
	i   int
	mu  sync.Mutex
}

func (f *fakeLister) ListUnfinishedIDs(_ context.Context) ([]uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.i >= len(f.ids) {
		return nil, nil
	}
	out := f.ids[f.i]
	f.i++
	return out, nil
}

type captureQueue struct {
	mu  sync.Mutex
	got []uuid.UUID
}

func (c *captureQueue) EnqueueProcessMeeting(_ context.Context, id uuid.UUID) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.got = append(c.got, id)
	return nil
}

func (c *captureQueue) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.got)
}

func TestSweeper_EnqueuesUnfinishedOnStartAndInterval(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	lister := &fakeLister{ids: [][]uuid.UUID{{a, b}, {c}}}
	q := &captureQueue{}
	s := NewSweeper(lister, q)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx, 30*time.Millisecond)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && q.len() < 3 {
		time.Sleep(10 * time.Millisecond)
	}
	if q.len() < 3 {
		t.Fatalf("enqueued %d, want 3 (startup sweep + interval sweep)", q.len())
	}
}

func TestSweeper_StopsOnContextCancel(t *testing.T) {
	lister := &fakeLister{}
	s := NewSweeper(lister, &captureQueue{})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx, 10*time.Millisecond)
		close(done)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop on cancel")
	}
}
