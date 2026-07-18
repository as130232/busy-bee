package meeting

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

// processFakeRepo 模擬 repo 狀態流轉，記錄呼叫軌跡。
type processFakeRepo struct {
	meeting       domainmeeting.Meeting
	transitions   []string
	savedText     string
	savedDuration int
	completedCall bool
	failedMessage string
}

func (f *processFakeRepo) Create(_ context.Context, m domainmeeting.Meeting) (domainmeeting.Meeting, error) {
	return m, nil
}
func (f *processFakeRepo) GetForUser(_ context.Context, _, _ uuid.UUID) (domainmeeting.Meeting, error) {
	return f.meeting, nil
}
func (f *processFakeRepo) Get(_ context.Context, _ uuid.UUID) (domainmeeting.Meeting, error) {
	return f.meeting, nil
}
func (f *processFakeRepo) UpdateStatus(_ context.Context, _ uuid.UUID, from, to domainmeeting.Status) (domainmeeting.Meeting, error) {
	if f.meeting.Status != from {
		return domainmeeting.Meeting{}, domainmeeting.ErrStatusConflict
	}
	f.transitions = append(f.transitions, string(from)+"->"+string(to))
	f.meeting.Status = to
	return f.meeting, nil
}
func (f *processFakeRepo) SaveTranscript(_ context.Context, _ uuid.UUID, text string, duration int) (domainmeeting.Meeting, error) {
	f.savedText, f.savedDuration = text, duration
	f.meeting.Transcript = text
	f.meeting.DurationSeconds = duration
	return f.meeting, nil
}
func (f *processFakeRepo) SetCompleted(_ context.Context, _ uuid.UUID) (domainmeeting.Meeting, error) {
	if f.meeting.Status != domainmeeting.StatusAnalyzing {
		return domainmeeting.Meeting{}, domainmeeting.ErrStatusConflict
	}
	f.completedCall = true
	f.meeting.Status = domainmeeting.StatusCompleted
	return f.meeting, nil
}
func (f *processFakeRepo) SetFailed(_ context.Context, _ uuid.UUID, msg string) (domainmeeting.Meeting, error) {
	f.failedMessage = msg
	f.meeting.Status = domainmeeting.StatusFailed
	f.meeting.ErrorMessage = msg
	return f.meeting, nil
}

type processFakeStorage struct {
	content     string
	downloaded  string
	downloadErr error
}

func (f *processFakeStorage) SignedUploadURL(_ context.Context, _, _ string, _ int64) (domainmeeting.UploadTarget, error) {
	return domainmeeting.UploadTarget{}, nil
}
func (f *processFakeStorage) Exists(_ context.Context, _ string) (bool, error) { return true, nil }
func (f *processFakeStorage) Download(_ context.Context, path string) (io.ReadCloser, int64, error) {
	f.downloaded = path
	if f.downloadErr != nil {
		return nil, 0, f.downloadErr
	}
	return io.NopCloser(strings.NewReader(f.content)), int64(len(f.content)), nil
}

type fakeSTT struct {
	result      domainmeeting.TranscribeResult
	err         error
	called      bool
	gotFilename string
}

func (f *fakeSTT) Transcribe(_ context.Context, _ io.Reader, _ int64, filename string) (domainmeeting.TranscribeResult, error) {
	f.called = true
	f.gotFilename = filename
	return f.result, f.err
}

func newProcessMeeting(status domainmeeting.Status, transcript string) domainmeeting.Meeting {
	return domainmeeting.Meeting{
		ID:           uuid.New(),
		UserID:       uuid.New(),
		Status:       status,
		Transcript:   transcript,
		AudioGCSPath: "audio/u/m.webm",
	}
}

type fakeNotifier struct {
	mu     sync.Mutex
	events []domainmeeting.StatusEvent
}

func (f *fakeNotifier) NotifyStatus(_ context.Context, e domainmeeting.StatusEvent) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, e)
}

func (f *fakeNotifier) statuses() []domainmeeting.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]domainmeeting.Status, len(f.events))
	for i, e := range f.events {
		out[i] = e.Status
	}
	return out
}

func TestProcess_FullPipelineFromPending(t *testing.T) {
	repo := &processFakeRepo{meeting: newProcessMeeting(domainmeeting.StatusPending, "")}
	st := &processFakeStorage{content: "audio-bytes"}
	stt := &fakeSTT{result: domainmeeting.TranscribeResult{Text: "會議逐字稿", DurationSeconds: 65}}
	uc := NewProcessUC(repo, st, stt, &fakeNotifier{})

	if err := uc.Execute(context.Background(), repo.meeting.ID); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	wantTransitions := []string{"pending->transcribing", "transcribing->analyzing"}
	if len(repo.transitions) != 2 || repo.transitions[0] != wantTransitions[0] || repo.transitions[1] != wantTransitions[1] {
		t.Errorf("transitions = %v, want %v", repo.transitions, wantTransitions)
	}
	if repo.savedText != "會議逐字稿" || repo.savedDuration != 65 {
		t.Errorf("saved transcript = %q dur=%d", repo.savedText, repo.savedDuration)
	}
	if !repo.completedCall {
		t.Error("SetCompleted not called")
	}
	if stt.gotFilename != "m.webm" {
		t.Errorf("stt filename = %q, want m.webm (base of gcs path)", stt.gotFilename)
	}
}

func TestProcess_CompletedIsIdempotentNoop(t *testing.T) {
	repo := &processFakeRepo{meeting: newProcessMeeting(domainmeeting.StatusCompleted, "done")}
	stt := &fakeSTT{}
	uc := NewProcessUC(repo, &processFakeStorage{}, stt, &fakeNotifier{})

	if err := uc.Execute(context.Background(), repo.meeting.ID); err != nil {
		t.Fatalf("Execute() on completed = %v, want nil", err)
	}
	if stt.called {
		t.Error("STT should not run for completed meeting")
	}
	if len(repo.transitions) != 0 {
		t.Errorf("transitions = %v, want none", repo.transitions)
	}
}

func TestProcess_RetrySkipsSTTWhenTranscriptExists(t *testing.T) {
	// 模擬上次跑到一半失敗重試：已在 transcribing 且 transcript 已存在
	repo := &processFakeRepo{meeting: newProcessMeeting(domainmeeting.StatusTranscribing, "既有逐字稿")}
	stt := &fakeSTT{}
	uc := NewProcessUC(repo, &processFakeStorage{}, stt, &fakeNotifier{})

	if err := uc.Execute(context.Background(), repo.meeting.ID); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if stt.called {
		t.Error("STT re-run despite existing transcript (breaks idempotency / double-billing)")
	}
	if !repo.completedCall {
		t.Error("pipeline should continue to completion")
	}
}

func TestProcess_STTErrorPropagates(t *testing.T) {
	repo := &processFakeRepo{meeting: newProcessMeeting(domainmeeting.StatusPending, "")}
	boom := errors.New("groq 500")
	uc := NewProcessUC(repo, &processFakeStorage{content: "x"}, &fakeSTT{err: boom}, &fakeNotifier{})

	err := uc.Execute(context.Background(), repo.meeting.ID)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want stt error propagated for asynq retry", err)
	}
	if repo.failedMessage != "" {
		t.Error("UC should not mark failed itself; that is the worker's last-retry decision")
	}
}

func TestProcess_ScheduledNotReady(t *testing.T) {
	repo := &processFakeRepo{meeting: newProcessMeeting(domainmeeting.StatusScheduled, "")}
	uc := NewProcessUC(repo, &processFakeStorage{}, &fakeSTT{}, &fakeNotifier{})

	if err := uc.Execute(context.Background(), repo.meeting.ID); err == nil {
		t.Fatal("Execute() on scheduled meeting should error (audio not confirmed)")
	}
}

func TestMarkFailed_RecordsMessage(t *testing.T) {
	repo := &processFakeRepo{meeting: newProcessMeeting(domainmeeting.StatusTranscribing, "")}
	uc := NewProcessUC(repo, &processFakeStorage{}, &fakeSTT{}, &fakeNotifier{})

	uc.MarkFailed(context.Background(), repo.meeting.ID, errors.New("groq unreachable"))

	if repo.failedMessage == "" {
		t.Fatal("SetFailed not called")
	}
}

func (f *processFakeRepo) ListUnfinishedIDs(_ context.Context) ([]uuid.UUID, error) {
	return nil, nil
}

func TestProcess_EmitsStatusEvents(t *testing.T) {
	repo := &processFakeRepo{meeting: newProcessMeeting(domainmeeting.StatusPending, "")}
	st := &processFakeStorage{content: "audio"}
	stt := &fakeSTT{result: domainmeeting.TranscribeResult{Text: "t", DurationSeconds: 5}}
	n := &fakeNotifier{}
	uc := NewProcessUC(repo, st, stt, n)

	if err := uc.Execute(context.Background(), repo.meeting.ID); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := []domainmeeting.Status{
		domainmeeting.StatusTranscribing,
		domainmeeting.StatusAnalyzing,
		domainmeeting.StatusCompleted,
	}
	got := n.statuses()
	if len(got) != len(want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event[%d] = %s, want %s", i, got[i], want[i])
		}
	}
	if n.events[0].UserID != repo.meeting.UserID {
		t.Error("event missing UserID (hub 需要它路由到正確用戶)")
	}
}

func TestMarkFailed_EmitsFailedEvent(t *testing.T) {
	repo := &processFakeRepo{meeting: newProcessMeeting(domainmeeting.StatusTranscribing, "")}
	n := &fakeNotifier{}
	uc := NewProcessUC(repo, &processFakeStorage{}, &fakeSTT{}, n)

	uc.MarkFailed(context.Background(), repo.meeting.ID, errors.New("boom"))

	got := n.statuses()
	if len(got) != 1 || got[0] != domainmeeting.StatusFailed {
		t.Fatalf("events = %v, want [failed]", got)
	}
	if n.events[0].ErrorMessage == "" {
		t.Error("failed event should carry error message")
	}
}
