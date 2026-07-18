package queue

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

// testRetryDelays 測試用極短 backoff。
var testRetryDelays = []time.Duration{10 * time.Millisecond, 20 * time.Millisecond}

func TestMemory_EnqueueAndConsume(t *testing.T) {
	var handled atomic.Int32
	var got uuid.UUID
	var mu sync.Mutex

	q := NewMemory(4, testRetryDelays)
	done := make(chan struct{})
	q.Start(context.Background(), 1, func(_ context.Context, id uuid.UUID) error {
		mu.Lock()
		got = id
		mu.Unlock()
		if handled.Add(1) == 1 {
			close(done)
		}
		return nil
	}, func(_ context.Context, _ uuid.UUID, _ error) {})
	t.Cleanup(func() { q.Stop(time.Second) })

	id := uuid.New()
	if err := q.EnqueueProcessMeeting(context.Background(), id); err != nil {
		t.Fatalf("Enqueue error = %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("task not consumed")
	}
	mu.Lock()
	defer mu.Unlock()
	if got != id {
		t.Errorf("handled id = %v, want %v", got, id)
	}
}

func TestMemory_DedupWhileInFlight(t *testing.T) {
	var calls atomic.Int32
	block := make(chan struct{})

	q := NewMemory(4, testRetryDelays)
	q.Start(context.Background(), 1, func(_ context.Context, _ uuid.UUID) error {
		calls.Add(1)
		<-block // 卡住模擬處理中
		return nil
	}, func(_ context.Context, _ uuid.UUID, _ error) {})
	t.Cleanup(func() { close(block); q.Stop(time.Second) })

	id := uuid.New()
	q.EnqueueProcessMeeting(context.Background(), id)
	time.Sleep(50 * time.Millisecond) // 等進入處理中

	// 處理中重複 enqueue：不報錯、不重複執行
	if err := q.EnqueueProcessMeeting(context.Background(), id); err != nil {
		t.Fatalf("duplicate enqueue error = %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if calls.Load() != 1 {
		t.Errorf("handler calls = %d, want 1 (dedup)", calls.Load())
	}
}

func TestMemory_ReEnqueueAfterCompletionWorks(t *testing.T) {
	var calls atomic.Int32
	q := NewMemory(4, testRetryDelays)
	q.Start(context.Background(), 1, func(_ context.Context, _ uuid.UUID) error {
		calls.Add(1)
		return nil
	}, func(_ context.Context, _ uuid.UUID, _ error) {})
	t.Cleanup(func() { q.Stop(time.Second) })

	id := uuid.New()
	q.EnqueueProcessMeeting(context.Background(), id)
	waitCond(t, func() bool { return calls.Load() == 1 })

	// 完成後再 enqueue 必須能再次執行（手動 retry 場景）
	q.EnqueueProcessMeeting(context.Background(), id)
	waitCond(t, func() bool { return calls.Load() == 2 })
}

func TestMemory_RetriesThenMarksFailed(t *testing.T) {
	var attempts atomic.Int32
	var failedID uuid.UUID
	var failedErr error
	failed := make(chan struct{})
	boom := errors.New("stt down")

	q := NewMemory(4, testRetryDelays)
	q.Start(context.Background(), 1, func(_ context.Context, _ uuid.UUID) error {
		attempts.Add(1)
		return boom
	}, func(_ context.Context, id uuid.UUID, err error) {
		failedID, failedErr = id, err
		close(failed)
	})
	t.Cleanup(func() { q.Stop(time.Second) })

	id := uuid.New()
	q.EnqueueProcessMeeting(context.Background(), id)

	select {
	case <-failed:
	case <-time.After(3 * time.Second):
		t.Fatal("onFail not called")
	}
	// 首次 + len(delays) 次重試
	if got := attempts.Load(); got != int32(1+len(testRetryDelays)) {
		t.Errorf("attempts = %d, want %d", got, 1+len(testRetryDelays))
	}
	if failedID != id || !errors.Is(failedErr, boom) {
		t.Errorf("onFail(%v, %v), want (%v, boom)", failedID, failedErr, id)
	}
}

func TestMemory_StopDrainsCurrentTask(t *testing.T) {
	started := make(chan struct{})
	finished := atomic.Bool{}

	q := NewMemory(4, testRetryDelays)
	q.Start(context.Background(), 1, func(_ context.Context, _ uuid.UUID) error {
		close(started)
		time.Sleep(100 * time.Millisecond)
		finished.Store(true)
		return nil
	}, func(_ context.Context, _ uuid.UUID, _ error) {})

	q.EnqueueProcessMeeting(context.Background(), uuid.New())
	<-started
	q.Stop(2 * time.Second)

	if !finished.Load() {
		t.Error("Stop() should wait for in-flight task to finish")
	}
}

func waitCond(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met within 2s")
}
