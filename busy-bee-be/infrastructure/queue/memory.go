// Package queue 提供背景任務佇列。
// Memory 為 in-process 實作（ADR-010）：業務狀態的真相源在 Postgres，
// 重啟遺失的任務由 worker.Sweeper 掃 DB 復原；分階段冪等（ADR-009）保證重跑無害。
package queue

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

// DefaultRetryDelays 各次重試的 backoff。
var DefaultRetryDelays = []time.Duration{30 * time.Second, 2 * time.Minute, 5 * time.Minute}

// Handler 處理單一會議任務；回錯誤觸發 retry。
type Handler func(ctx context.Context, meetingID uuid.UUID) error

// OnFail 最後一次重試仍失敗時呼叫（標記 failed）。
type OnFail func(ctx context.Context, meetingID uuid.UUID, err error)

type task struct {
	meetingID uuid.UUID
	attempt   int // 0 = 首次
}

type Memory struct {
	tasks       chan task
	retryDelays []time.Duration
	taskTimeout time.Duration // 單一任務逾時上界；<=0 表示不設限

	mu       sync.Mutex
	inFlight map[uuid.UUID]struct{} // 排隊中或執行中（含等待 retry）

	stop    chan struct{}
	wg      sync.WaitGroup
	timerWG sync.WaitGroup
}

var _ domainmeeting.TaskQueue = (*Memory)(nil)

// NewMemory 建立 in-process 佇列。taskTimeout 為單一任務的執行上界（<=0 不設限），
// 避免外部 API（STT/LLM）卡死無限期佔用有限的 worker 名額。
func NewMemory(buffer int, taskTimeout time.Duration, retryDelays []time.Duration) *Memory {
	return &Memory{
		tasks:       make(chan task, buffer),
		retryDelays: retryDelays,
		taskTimeout: taskTimeout,
		inFlight:    make(map[uuid.UUID]struct{}),
		stop:        make(chan struct{}),
	}
}

// EnqueueProcessMeeting 排入任務。同會議在排隊/執行/等待重試期間重複排入為冪等 no-op；
// 佇列滿時回錯誤（背壓，API 端轉 503）。
func (m *Memory) EnqueueProcessMeeting(_ context.Context, meetingID uuid.UUID) error {
	m.mu.Lock()
	if _, exists := m.inFlight[meetingID]; exists {
		m.mu.Unlock()
		return nil
	}
	m.inFlight[meetingID] = struct{}{}
	m.mu.Unlock()

	select {
	case m.tasks <- task{meetingID: meetingID}:
		return nil
	default:
		m.release(meetingID)
		return fmt.Errorf("queue.Memory: queue full (%d)", cap(m.tasks))
	}
}

// Start 啟動 worker pool。handler 錯誤依 retryDelays 重試，仍失敗則呼叫 onFail。
func (m *Memory) Start(ctx context.Context, workers int, handler Handler, onFail OnFail) {
	for i := 0; i < workers; i++ {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			for {
				select {
				case <-m.stop:
					return
				case t := <-m.tasks:
					m.run(ctx, t, handler, onFail)
				}
			}
		}()
	}
}

func (m *Memory) run(ctx context.Context, t task, handler Handler, onFail OnFail) {
	err := m.invoke(ctx, t, handler)
	if err == nil {
		m.release(t.meetingID)
		return
	}

	if t.attempt >= len(m.retryDelays) {
		slog.ErrorContext(ctx, "queue.memory.exhausted",
			"meeting_id", t.meetingID, "attempts", t.attempt+1, "err", err)
		onFail(ctx, t.meetingID, err)
		m.release(t.meetingID)
		return
	}

	delay := m.retryDelays[t.attempt]
	slog.WarnContext(ctx, "queue.memory.retry_scheduled",
		"meeting_id", t.meetingID, "attempt", t.attempt+1, "delay", delay.String(), "err", err)

	// 等待重試期間仍佔用 inFlight（維持去重）；Stop 時放棄等待，交給 Sweeper 復原
	m.timerWG.Add(1)
	go func() {
		defer m.timerWG.Done()
		select {
		case <-m.stop:
			m.release(t.meetingID)
		case <-time.After(delay):
			select {
			case m.tasks <- task{meetingID: t.meetingID, attempt: t.attempt + 1}:
			case <-m.stop:
				m.release(t.meetingID)
			}
		}
	}()
}

// invoke 執行 handler，套用單任務逾時並攔截 panic。
// panic 轉成一般 error 交由 retry/onFail 機制處理，避免單一任務讓整個 process 崩潰
// （單 instance 設計下，未攔截的 panic 等於全面 outage）。
func (m *Memory) invoke(ctx context.Context, t task, handler Handler) (err error) {
	if m.taskTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.taskTimeout)
		defer cancel()
	}

	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "queue.memory.panic",
				"meeting_id", t.meetingID, "attempt", t.attempt+1,
				"panic", r, "stack", string(debug.Stack()))
			err = fmt.Errorf("queue.Memory: handler panic: %v", r)
		}
	}()

	return handler(ctx, t.meetingID)
}

func (m *Memory) release(id uuid.UUID) {
	m.mu.Lock()
	delete(m.inFlight, id)
	m.mu.Unlock()
}

// Stop 停止收工並等待進行中任務結束（至多 timeout）。
// 未完成的任務由下次啟動的 Sweeper 從 DB 復原。
func (m *Memory) Stop(timeout time.Duration) {
	close(m.stop)
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		m.timerWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		slog.Warn("queue.memory.stop_timeout: in-flight tasks abandoned, sweeper will recover")
	}
}
