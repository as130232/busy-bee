package meeting

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	domainactionitem "github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	domainartifact "github.com/as130232/busy-bee/busy-bee-be/domain/artifact"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

// processFakeRepo 模擬 repo 狀態流轉，記錄呼叫軌跡。
type processFakeRepo struct {
	meeting       domainmeeting.Meeting
	transitions   []string
	savedText     string
	savedSegments []domainmeeting.TranscriptSegment
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
func (f *processFakeRepo) SaveTranscript(_ context.Context, _ uuid.UUID, text string, segments []domainmeeting.TranscriptSegment, duration int) (domainmeeting.Meeting, error) {
	f.savedText, f.savedSegments, f.savedDuration = text, segments, duration
	f.meeting.Transcript = text
	f.meeting.TranscriptSegments = segments
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
func (f *processFakeStorage) SignedDownloadURL(_ context.Context, _ string) (string, error) {
	return "https://signed-download", nil
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
	uc := newTestProcessUC(repo, st, stt, &fakeNotifier{})

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
	uc := newTestProcessUC(repo, &processFakeStorage{}, stt, &fakeNotifier{})

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
	uc := newTestProcessUC(repo, &processFakeStorage{}, stt, &fakeNotifier{})

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
	uc := newTestProcessUC(repo, &processFakeStorage{content: "x"}, &fakeSTT{err: boom}, &fakeNotifier{})

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
	uc := newTestProcessUC(repo, &processFakeStorage{}, &fakeSTT{}, &fakeNotifier{})

	if err := uc.Execute(context.Background(), repo.meeting.ID); err == nil {
		t.Fatal("Execute() on scheduled meeting should error (audio not confirmed)")
	}
}

func TestMarkFailed_RecordsMessage(t *testing.T) {
	repo := &processFakeRepo{meeting: newProcessMeeting(domainmeeting.StatusTranscribing, "")}
	uc := newTestProcessUC(repo, &processFakeStorage{}, &fakeSTT{}, &fakeNotifier{})

	uc.MarkFailed(context.Background(), repo.meeting.ID, errors.New("groq unreachable"))

	if repo.failedMessage == "" {
		t.Fatal("SetFailed not called")
	}
}

// error_message 會經 API / WS 回給前端，禁止含外部錯誤原文（資料安全規範）。
func TestMarkFailed_DoesNotExposeRawCause(t *testing.T) {
	repo := &processFakeRepo{meeting: newProcessMeeting(domainmeeting.StatusTranscribing, "")}
	uc := newTestProcessUC(repo, &processFakeStorage{}, &fakeSTT{}, &fakeNotifier{})

	rawCause := errors.New(`process transcribe: groq status 500: {"error":{"message":"internal","url":"https://api.groq.com/openai/v1/audio"}}`)
	uc.MarkFailed(context.Background(), repo.meeting.ID, rawCause)

	if strings.Contains(repo.failedMessage, "groq") || strings.Contains(repo.failedMessage, "api.groq.com") {
		t.Errorf("error_message 不得含外部錯誤原文, got %q", repo.failedMessage)
	}
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
	uc := newTestProcessUC(repo, st, stt, n)

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
	uc := newTestProcessUC(repo, &processFakeStorage{}, &fakeSTT{}, n)

	uc.MarkFailed(context.Background(), repo.meeting.ID, errors.New("boom"))

	got := n.statuses()
	if len(got) != 1 || got[0] != domainmeeting.StatusFailed {
		t.Fatalf("events = %v, want [failed]", got)
	}
	if n.events[0].ErrorMessage == "" {
		t.Error("failed event should carry error message")
	}
}

// --- Phase 9：analyzing 階段 fakes 與測試 ---

type fakeArtifactRepo struct {
	mu       sync.Mutex
	existing []domainartifact.Artifact
	saved    map[domainartifact.Type]string
}

func (f *fakeArtifactRepo) Upsert(_ context.Context, meetingID uuid.UUID, t domainartifact.Type, content string) (domainartifact.Artifact, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saved == nil {
		f.saved = make(map[domainartifact.Type]string)
	}
	f.saved[t] = content
	return domainartifact.Artifact{MeetingID: meetingID, Type: t, Content: content}, nil
}

func (f *fakeArtifactRepo) ListByMeeting(_ context.Context, _ uuid.UUID) ([]domainartifact.Artifact, error) {
	return f.existing, nil
}

type fakeLLM struct {
	prdCalls  int
	specCalls int
	err       error
}

func (f *fakeLLM) GeneratePRD(_ context.Context, transcript string) (string, error) {
	f.prdCalls++
	return "# PRD from: " + transcript[:min(10, len(transcript))], f.err
}

func (f *fakeLLM) GenerateTechSpec(_ context.Context, _ string) (string, error) {
	f.specCalls++
	return "# Tech Spec", f.err
}

func newTestProcessUC(repo *processFakeRepo, st *processFakeStorage, stt *fakeSTT, n *fakeNotifier) *ProcessUC {
	return NewProcessUC(ProcessDeps{
		Meetings: repo, Storage: st, STT: stt,
		Artifacts: &fakeArtifactRepo{}, LLM: &fakeLLM{}, Notifier: n,
		ActionItems: &fakeActionItemRepo{}, Extractor: &fakeExtractor{},
	})
}

// --- Phase 13：行動項抽取 fakes 與測試 ---

type fakeActionItemRepo struct {
	mu           sync.Mutex
	inserted     []domainactionitem.Extracted
	deleteCalled bool
}

func (f *fakeActionItemRepo) Insert(_ context.Context, meetingID, userID uuid.UUID, item domainactionitem.Extracted, _ int) (domainactionitem.ActionItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inserted = append(f.inserted, item)
	return domainactionitem.ActionItem{MeetingID: meetingID, UserID: userID, Description: item.Description}, nil
}

func (f *fakeActionItemRepo) DeleteForMeeting(_ context.Context, _ uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteCalled = true
	return nil
}

func (f *fakeActionItemRepo) ListByMeeting(_ context.Context, _ uuid.UUID) ([]domainactionitem.ActionItem, error) {
	return nil, nil
}

func (f *fakeActionItemRepo) ListPendingForUser(_ context.Context, _ uuid.UUID) ([]domainactionitem.PendingItem, error) {
	return nil, nil
}

func (f *fakeActionItemRepo) SetDone(_ context.Context, _, _ uuid.UUID, _ bool) (domainactionitem.ActionItem, error) {
	return domainactionitem.ActionItem{}, nil
}

type fakeExtractor struct {
	items  []domainactionitem.Extracted
	err    error
	called int
}

func (f *fakeExtractor) ExtractActionItems(_ context.Context, _ string) ([]domainactionitem.Extracted, error) {
	f.called++
	return f.items, f.err
}

const artifactTypeActionItemsTest = domainartifact.Type("action_items")

func TestProcess_ExtractsActionItems(t *testing.T) {
	repo := &processFakeRepo{meeting: newProcessMeeting(domainmeeting.StatusAnalyzing, "逐字稿內容")}
	arts := &fakeArtifactRepo{}
	items := &fakeActionItemRepo{}
	ext := &fakeExtractor{items: []domainactionitem.Extracted{{Description: "做 A"}, {Description: "做 B"}}}
	uc := NewProcessUC(ProcessDeps{
		Meetings: repo, Storage: &processFakeStorage{}, STT: &fakeSTT{},
		Artifacts: arts, LLM: &fakeLLM{}, Notifier: &fakeNotifier{},
		ActionItems: items, Extractor: ext,
	})

	if err := uc.Execute(context.Background(), repo.meeting.ID); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if ext.called != 1 {
		t.Errorf("extractor called %d times, want 1", ext.called)
	}
	if !items.deleteCalled {
		t.Error("DeleteForMeeting should be called before insert (avoid dup on retry)")
	}
	if len(items.inserted) != 2 {
		t.Errorf("inserted %d items, want 2", len(items.inserted))
	}
	if arts.saved[artifactTypeActionItemsTest] == "" {
		t.Error("action_items marker not saved to artifacts")
	}
	if !repo.completedCall {
		t.Error("pipeline should complete")
	}
}

func TestProcess_ActionItemsIdempotentWhenMarkerExists(t *testing.T) {
	repo := &processFakeRepo{meeting: newProcessMeeting(domainmeeting.StatusAnalyzing, "t")}
	arts := &fakeArtifactRepo{existing: []domainartifact.Artifact{
		{Type: domainartifact.TypePRD}, {Type: domainartifact.TypeTechSpec}, {Type: artifactTypeActionItemsTest},
	}}
	items := &fakeActionItemRepo{}
	ext := &fakeExtractor{}
	uc := NewProcessUC(ProcessDeps{
		Meetings: repo, Storage: &processFakeStorage{}, STT: &fakeSTT{},
		Artifacts: arts, LLM: &fakeLLM{}, Notifier: &fakeNotifier{},
		ActionItems: items, Extractor: ext,
	})

	if err := uc.Execute(context.Background(), repo.meeting.ID); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if ext.called != 0 {
		t.Errorf("extractor called %d times, want 0 (already extracted)", ext.called)
	}
}

func TestProcess_ActionItemsEmptyStillMarks(t *testing.T) {
	repo := &processFakeRepo{meeting: newProcessMeeting(domainmeeting.StatusAnalyzing, "t")}
	arts := &fakeArtifactRepo{}
	items := &fakeActionItemRepo{}
	ext := &fakeExtractor{items: nil} // 會議無行動項
	uc := NewProcessUC(ProcessDeps{
		Meetings: repo, Storage: &processFakeStorage{}, STT: &fakeSTT{},
		Artifacts: arts, LLM: &fakeLLM{}, Notifier: &fakeNotifier{},
		ActionItems: items, Extractor: ext,
	})

	if err := uc.Execute(context.Background(), repo.meeting.ID); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(items.inserted) != 0 {
		t.Errorf("inserted %d items, want 0", len(items.inserted))
	}
	if arts.saved[artifactTypeActionItemsTest] == "" {
		t.Error("marker should still be saved even with no items (avoid re-extraction on retry)")
	}
	if !repo.completedCall {
		t.Error("should complete")
	}
}

func TestProcess_ActionItemExtractErrorPropagates(t *testing.T) {
	repo := &processFakeRepo{meeting: newProcessMeeting(domainmeeting.StatusAnalyzing, "t")}
	ext := &fakeExtractor{err: errors.New("gemini 500")}
	uc := NewProcessUC(ProcessDeps{
		Meetings: repo, Storage: &processFakeStorage{}, STT: &fakeSTT{},
		Artifacts: &fakeArtifactRepo{}, LLM: &fakeLLM{}, Notifier: &fakeNotifier{},
		ActionItems: &fakeActionItemRepo{}, Extractor: ext,
	})

	if err := uc.Execute(context.Background(), repo.meeting.ID); err == nil {
		t.Fatal("Execute() should propagate extractor error for retry")
	}
	if repo.completedCall {
		t.Error("must not complete when extraction failed")
	}
}

func TestProcess_AnalyzingGeneratesBothDocs(t *testing.T) {
	repo := &processFakeRepo{meeting: newProcessMeeting(domainmeeting.StatusAnalyzing, "逐字稿內容在此")}
	arts := &fakeArtifactRepo{}
	llm := &fakeLLM{}
	uc := NewProcessUC(ProcessDeps{
		Meetings: repo, Storage: &processFakeStorage{}, STT: &fakeSTT{},
		Artifacts: arts, LLM: llm, Notifier: &fakeNotifier{},
		ActionItems: &fakeActionItemRepo{}, Extractor: &fakeExtractor{},
	})

	if err := uc.Execute(context.Background(), repo.meeting.ID); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if llm.prdCalls != 1 || llm.specCalls != 1 {
		t.Errorf("llm calls prd=%d spec=%d, want 1/1", llm.prdCalls, llm.specCalls)
	}
	if arts.saved[domainartifact.TypePRD] == "" || arts.saved[domainartifact.TypeTechSpec] == "" {
		t.Errorf("saved = %v, want both docs", arts.saved)
	}
	if !repo.completedCall {
		t.Error("should complete after generation")
	}
}

func TestProcess_AnalyzingSkipsExistingDoc(t *testing.T) {
	// retry 場景：PRD 已生成，只補 Tech Spec（不重複扣費）
	repo := &processFakeRepo{meeting: newProcessMeeting(domainmeeting.StatusAnalyzing, "t")}
	arts := &fakeArtifactRepo{existing: []domainartifact.Artifact{{Type: domainartifact.TypePRD, Content: "old"}}}
	llm := &fakeLLM{}
	uc := NewProcessUC(ProcessDeps{
		Meetings: repo, Storage: &processFakeStorage{}, STT: &fakeSTT{},
		Artifacts: arts, LLM: llm, Notifier: &fakeNotifier{},
		ActionItems: &fakeActionItemRepo{}, Extractor: &fakeExtractor{},
	})

	if err := uc.Execute(context.Background(), repo.meeting.ID); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if llm.prdCalls != 0 {
		t.Errorf("prdCalls = %d, want 0 (already exists)", llm.prdCalls)
	}
	if llm.specCalls != 1 {
		t.Errorf("specCalls = %d, want 1", llm.specCalls)
	}
}

func TestProcess_LLMErrorPropagatesWithoutCompletion(t *testing.T) {
	repo := &processFakeRepo{meeting: newProcessMeeting(domainmeeting.StatusAnalyzing, "t")}
	llm := &fakeLLM{err: errors.New("gemini 429")}
	uc := NewProcessUC(ProcessDeps{
		Meetings: repo, Storage: &processFakeStorage{}, STT: &fakeSTT{},
		Artifacts: &fakeArtifactRepo{}, LLM: llm, Notifier: &fakeNotifier{},
	})

	if err := uc.Execute(context.Background(), repo.meeting.ID); err == nil {
		t.Fatal("Execute() should propagate llm error for retry")
	}
	if repo.completedCall {
		t.Error("must not complete when generation failed")
	}
}

func (f *processFakeRepo) ListForUser(_ context.Context, _ uuid.UUID, _ string) ([]domainmeeting.Meeting, error) {
	return []domainmeeting.Meeting{f.meeting}, nil
}
