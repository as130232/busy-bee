package correction

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	"github.com/as130232/busy-bee/busy-bee-be/domain/correction"
	"github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainmc "github.com/as130232/busy-bee/busy-bee-be/domain/meetingcontext"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// ---- fakes ----

type fakeRepo struct {
	mu        sync.Mutex
	createErr error
	gotSnap   correction.Snapshot
	created   correction.Correction
	stored    map[uuid.UUID]correction.Correction
	latest    *correction.Correction
	latestErr error
	count     int
	progress  []correction.Progress
	applied   *correction.Result
	// current 模擬 DB 落庫交易內讀到的當下 segments；appliedSegs 為合併結果
	current     func() []meeting.TranscriptSegment
	appliedSegs []meeting.TranscriptSegment
	reverted    bool
	statuses    []correction.Status
}

func newFakeRepo() *fakeRepo { return &fakeRepo{stored: map[uuid.UUID]correction.Correction{}} }

func (f *fakeRepo) CreateWithSnapshot(_ context.Context, meetingID, userID uuid.UUID, snap correction.Snapshot) (correction.Correction, error) {
	if f.createErr != nil {
		return correction.Correction{}, f.createErr
	}
	f.gotSnap = snap
	f.created = correction.Correction{ID: uuid.New(), MeetingID: meetingID, UserID: userID, Attempt: f.count + 1, Status: correction.StatusQueued, Original: snap}
	return f.created, nil
}
func (f *fakeRepo) Get(_ context.Context, id uuid.UUID) (correction.Correction, error) {
	c, ok := f.stored[id]
	if !ok {
		return correction.Correction{}, correction.ErrNotFound
	}
	return c, nil
}
func (f *fakeRepo) GetLatestForMeeting(context.Context, uuid.UUID, uuid.UUID) (correction.Correction, error) {
	if f.latestErr != nil {
		return correction.Correction{}, f.latestErr
	}
	if f.latest == nil {
		return correction.Correction{}, correction.ErrNotFound
	}
	return *f.latest, nil
}
func (f *fakeRepo) CountForMeeting(context.Context, uuid.UUID, uuid.UUID) (int, error) {
	return f.count, nil
}
func (f *fakeRepo) UpdateStatus(_ context.Context, id uuid.UUID, from, to correction.Status, msg string) (correction.Correction, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.stored[id]
	if !ok || c.Status != from {
		return correction.Correction{}, correction.ErrStatusConflict
	}
	c.Status, c.ErrorMessage = to, msg
	f.stored[id] = c
	f.statuses = append(f.statuses, to)
	return c, nil
}
func (f *fakeRepo) SaveProgress(_ context.Context, _ uuid.UUID, p correction.Progress) error {
	cp := correction.Progress{Chunks: map[string][]correction.Patch{}}
	for k, v := range p.Chunks {
		cp.Chunks[k] = v
	}
	f.progress = append(f.progress, cp)
	return nil
}
func (f *fakeRepo) ListUnfinishedIDs(context.Context) ([]uuid.UUID, error) { return nil, nil }
func (f *fakeRepo) ApplyResult(_ context.Context, c correction.Correction, r correction.Result) error {
	f.applied = &r
	if f.current != nil {
		f.appliedSegs = correction.ApplyPatches(f.current(), r.Patches)
	}
	c.Status = correction.StatusCompleted
	f.stored[c.ID] = c
	return nil
}
func (f *fakeRepo) Revert(context.Context, correction.Correction) error {
	f.reverted = true
	return nil
}

type fakeMeetings struct {
	m   meeting.Meeting
	err error
	// afterFirst 第二次（含）之後的 GetForUser 回傳此值，模擬校正期間使用者編輯
	afterFirst *meeting.Meeting
	calls      int
}

func (f *fakeMeetings) GetForUser(context.Context, uuid.UUID, uuid.UUID) (meeting.Meeting, error) {
	f.calls++
	if f.err != nil {
		return meeting.Meeting{}, f.err
	}
	if f.afterFirst != nil && f.calls > 1 {
		return *f.afterFirst, nil
	}
	return f.m, nil
}

type fakeQueue struct{ ids []uuid.UUID }

func (f *fakeQueue) EnqueueCorrection(_ context.Context, id uuid.UUID) error {
	f.ids = append(f.ids, id)
	return nil
}

type fakeIndexer struct{ called int }

func (f *fakeIndexer) Execute(context.Context, uuid.UUID) error {
	f.called++
	return nil
}

type fakeStorage struct{}

func (fakeStorage) Download(context.Context, string) (io.ReadCloser, int64, error) {
	return io.NopCloser(strings.NewReader("audio")), 5, nil
}

type fakeClipper struct{ calls int }

func (f *fakeClipper) CutClip(context.Context, string, int, int) (string, func(), error) {
	f.calls++
	return "clip.mp3", func() {}, nil
}

type fakeCorrector struct {
	byFirst map[int][]correction.Patch // chunk 首段索引 → 回傳
	err     error
	calls   []correction.CorrectorInput
}

func (f *fakeCorrector) CorrectChunk(_ context.Context, in correction.CorrectorInput) ([]correction.Patch, error) {
	f.calls = append(f.calls, in)
	if f.err != nil {
		return nil, f.err
	}
	return f.byFirst[in.Segments[0].I], nil
}

type fakeSummarizer struct {
	sections []meeting.SummarySection
	gotBg    string
}

func (f *fakeSummarizer) Summarize(_ context.Context, _ string, _ meeting.Scenario, background string) (meeting.SummaryResult, error) {
	f.gotBg = background
	return meeting.SummaryResult{Sections: f.sections, Tags: []string{"不應寫入"}}, nil
}

type fakeExtractor struct {
	out         actionitem.Extraction
	gotExisting []string
	gotBg       string
}

func (f *fakeExtractor) Extract(ctx context.Context, tr string, now time.Time, background string) (actionitem.Extraction, error) {
	return f.ExtractExcluding(ctx, tr, now, nil, background)
}
func (f *fakeExtractor) ExtractExcluding(_ context.Context, _ string, _ time.Time, existing []string, background string) (actionitem.Extraction, error) {
	f.gotExisting = existing
	f.gotBg = background
	return f.out, nil
}

type fakeContextLister struct{ list []domainmc.Context }

func (f *fakeContextLister) List(context.Context, uuid.UUID, uuid.UUID) ([]domainmc.Context, error) {
	return f.list, nil
}

type fakeItems struct{ items []actionitem.ActionItem }

func (f *fakeItems) ListByMeeting(context.Context, uuid.UUID) ([]actionitem.ActionItem, error) {
	return f.items, nil
}

type fakeNotifier struct{ events []meeting.StatusEvent }

func (f *fakeNotifier) NotifyStatus(_ context.Context, e meeting.StatusEvent) {
	f.events = append(f.events, e)
}

func assertErrCode(t *testing.T, err error, want errcode.ErrCode) {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != want {
		t.Fatalf("err = %v, want code %d", err, want)
	}
}

var (
	ctx = context.Background()
	uid = uuid.New()
	mid = uuid.New()
)

func completedMeeting() meeting.Meeting {
	return meeting.Meeting{
		ID: mid, UserID: uid, Status: meeting.StatusCompleted, AudioGCSPath: "u/a.webm",
		Transcript:         "A: 小娃處理",
		TranscriptSegments: []meeting.TranscriptSegment{{Speaker: "A", Text: "小娃處理", StartMs: 0, EndMs: 1000}},
		SummarySections:    []meeting.SummarySection{{Type: "summary", Title: "摘要"}},
		Summary:            "舊摘要",
	}
}

// ---- Start ----

func TestStart_RejectsNotCompleted(t *testing.T) {
	m := completedMeeting()
	m.Status = meeting.StatusAnalyzing
	_, err := NewStartUC(&fakeMeetings{m: m}, newFakeRepo(), &fakeQueue{}).Execute(ctx, uid, mid)
	assertErrCode(t, err, errcode.CorrectionNotAllowed)
}

func TestStart_RejectsNoAudio(t *testing.T) {
	m := completedMeeting()
	m.AudioGCSPath = ""
	_, err := NewStartUC(&fakeMeetings{m: m}, newFakeRepo(), &fakeQueue{}).Execute(ctx, uid, mid)
	assertErrCode(t, err, errcode.CorrectionNotAllowed)
}

func TestStart_NotFound(t *testing.T) {
	_, err := NewStartUC(&fakeMeetings{err: meeting.ErrNotFound}, newFakeRepo(), &fakeQueue{}).Execute(ctx, uid, mid)
	assertErrCode(t, err, errcode.NotFound)
}

func TestStart_LimitReached(t *testing.T) {
	repo := newFakeRepo()
	repo.createErr = correction.ErrStatusConflict
	_, err := NewStartUC(&fakeMeetings{m: completedMeeting()}, repo, &fakeQueue{}).Execute(ctx, uid, mid)
	assertErrCode(t, err, errcode.CorrectionLimitReached)
}

func TestStart_InProgress(t *testing.T) {
	repo := newFakeRepo()
	repo.createErr = correction.ErrActiveExists
	_, err := NewStartUC(&fakeMeetings{m: completedMeeting()}, repo, &fakeQueue{}).Execute(ctx, uid, mid)
	assertErrCode(t, err, errcode.CorrectionInProgress)
}

func TestStart_SnapshotsAndEnqueues(t *testing.T) {
	m := completedMeeting()
	repo, q := newFakeRepo(), &fakeQueue{}
	st, err := NewStartUC(&fakeMeetings{m: m}, repo, q).Execute(ctx, uid, mid)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if repo.gotSnap.Summary != "舊摘要" || repo.gotSnap.Transcript != m.Transcript ||
		len(repo.gotSnap.Segments) != 1 || len(repo.gotSnap.SummarySections) != 1 {
		t.Errorf("snapshot = %+v", repo.gotSnap)
	}
	if len(q.ids) != 1 || q.ids[0] != repo.created.ID {
		t.Errorf("enqueued = %v", q.ids)
	}
	if st.Status != correction.StatusQueued || st.AttemptsUsed != 1 || st.CanRevert {
		t.Errorf("state = %+v", st)
	}
}

// ---- Get ----

func TestGet_NoCorrection(t *testing.T) {
	st, err := NewGetUC(&fakeMeetings{m: completedMeeting()}, newFakeRepo()).Execute(ctx, uid, mid)
	if err != nil || st != (correction.State{}) {
		t.Errorf("state = %+v err %v", st, err)
	}
}

func TestGet_OwnerCheck(t *testing.T) {
	_, err := NewGetUC(&fakeMeetings{err: meeting.ErrNotFound}, newFakeRepo()).Execute(ctx, uid, mid)
	assertErrCode(t, err, errcode.NotFound)
}

func TestGet_ReturnsLatest(t *testing.T) {
	repo := newFakeRepo()
	repo.latest = &correction.Correction{Status: correction.StatusCompleted}
	repo.count = 1
	st, err := NewGetUC(&fakeMeetings{m: completedMeeting()}, repo).Execute(ctx, uid, mid)
	if err != nil || !st.CanRevert || st.AttemptsUsed != 1 {
		t.Errorf("state = %+v err %v", st, err)
	}
}

// ---- Revert ----

func TestRevert_OnlyLatestCompleted(t *testing.T) {
	repo := newFakeRepo()
	repo.latest = &correction.Correction{Status: correction.StatusFailed}
	_, err := NewRevertUC(repo, &fakeIndexer{}).Execute(ctx, uid, mid)
	assertErrCode(t, err, errcode.NothingToRevert)
	if repo.reverted {
		t.Error("不應還原")
	}
}

func TestRevert_NoCorrection(t *testing.T) {
	_, err := NewRevertUC(newFakeRepo(), &fakeIndexer{}).Execute(ctx, uid, mid)
	assertErrCode(t, err, errcode.NothingToRevert)
}

func TestRevert_RevertsAndReindexes(t *testing.T) {
	repo, idx := newFakeRepo(), &fakeIndexer{}
	repo.latest = &correction.Correction{ID: uuid.New(), MeetingID: mid, UserID: uid, Status: correction.StatusCompleted}
	repo.count = 1
	st, err := NewRevertUC(repo, idx).Execute(ctx, uid, mid)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !repo.reverted || idx.called != 1 || st.Status != correction.StatusReverted || st.CanRevert {
		t.Errorf("reverted=%v reindex=%d state=%+v", repo.reverted, idx.called, st)
	}
}

// ---- Execute ----

type execFixture struct {
	repo      *fakeRepo
	meetings  *fakeMeetings
	corrector *fakeCorrector
	clipper   *fakeClipper
	extractor *fakeExtractor
	summarize *fakeSummarizer
	contexts  *fakeContextLister
	items     *fakeItems
	indexer   *fakeIndexer
	notifier  *fakeNotifier
	c         correction.Correction
	uc        *ExecuteUC
}

// newExecFixture 以 segs 建一筆 queued 校正（快照 = segs）。
func newExecFixture(segs []meeting.TranscriptSegment) *execFixture {
	m := completedMeeting()
	m.TranscriptSegments = segs
	f := &execFixture{
		repo: newFakeRepo(), meetings: &fakeMeetings{m: m},
		corrector: &fakeCorrector{byFirst: map[int][]correction.Patch{}}, clipper: &fakeClipper{},
		extractor: &fakeExtractor{}, items: &fakeItems{}, indexer: &fakeIndexer{}, notifier: &fakeNotifier{},
		summarize: &fakeSummarizer{sections: []meeting.SummarySection{{Type: "summary", Title: "新"}}},
		contexts:  &fakeContextLister{},
	}
	f.c = correction.Correction{
		ID: uuid.New(), MeetingID: mid, UserID: uid, Attempt: 1, Status: correction.StatusQueued,
		Progress: correction.Progress{Chunks: map[string][]correction.Patch{}},
		Original: correction.Snapshot{Segments: segs},
	}
	f.repo.stored[f.c.ID] = f.c
	f.repo.current = func() []meeting.TranscriptSegment {
		if f.meetings.afterFirst != nil {
			return f.meetings.afterFirst.TranscriptSegments
		}
		return f.meetings.m.TranscriptSegments
	}
	f.uc = NewExecuteUC(ExecuteDeps{
		Repo: f.repo, Meetings: f.meetings, Storage: fakeStorage{}, Clipper: f.clipper,
		Corrector: f.corrector, Summarizer: f.summarize, Contexts: f.contexts,
		Extractor: f.extractor, Items: f.items, Indexer: f.indexer, Notifier: f.notifier,
		Compress: func(context.Context, io.Reader) (string, func(), error) { return "full.mp3", func() {}, nil },
		OpenClip: func(string) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("clip")), nil },
	})
	return f
}

// thirtyMinSegs 6 段各 5 分鐘 → PlanChunks 切成 3 段（[0,1] [2,3] [4,5]）。
func thirtyMinSegs() []meeting.TranscriptSegment {
	segs := make([]meeting.TranscriptSegment, 6)
	for i := range segs {
		segs[i] = meeting.TranscriptSegment{Speaker: "A", Text: fmt.Sprintf("原文%d", i), StartMs: i * 300_000, EndMs: (i + 1) * 300_000}
	}
	return segs
}

// 會前資料（F-LIVE）：校正每段、重產摘要與重抽行動項都帶入背景資料。
func TestExecute_PassesMeetingContextsAsBackground(t *testing.T) {
	f := newExecFixture(thirtyMinSegs())
	f.contexts.list = []domainmc.Context{{Title: "名詞表", Content: "Stage、Jenkins"}}
	if err := f.uc.Execute(ctx, f.c.ID); err != nil {
		t.Fatalf("err = %v", err)
	}
	want := domainmc.PromptBlock(f.contexts.list)
	for i, call := range f.corrector.calls {
		if call.Background != want {
			t.Errorf("chunk %d background = %q", i, call.Background)
		}
	}
	if f.summarize.gotBg != want || f.extractor.gotBg != want {
		t.Errorf("summary bg=%q extract bg=%q", f.summarize.gotBg, f.extractor.gotBg)
	}
}

func TestExecute_SkipsTerminal(t *testing.T) {
	for _, s := range []correction.Status{correction.StatusCompleted, correction.StatusFailed, correction.StatusReverted} {
		f := newExecFixture(thirtyMinSegs())
		f.c.Status = s
		f.repo.stored[f.c.ID] = f.c
		if err := f.uc.Execute(ctx, f.c.ID); err != nil {
			t.Fatalf("%s: err = %v", s, err)
		}
		if len(f.corrector.calls) != 0 || f.repo.applied != nil {
			t.Errorf("%s: 終態不應再處理", s)
		}
	}
}

func TestExecute_MissingCorrectionIsNoop(t *testing.T) {
	f := newExecFixture(thirtyMinSegs())
	if err := f.uc.Execute(ctx, uuid.New()); err != nil {
		t.Errorf("err = %v", err)
	}
}

func TestExecute_SkipsCompletedChunks(t *testing.T) {
	f := newExecFixture(thirtyMinSegs())
	f.c.Progress.Chunks["0"] = []correction.Patch{{I: 0, Text: "已校正0"}}
	f.repo.stored[f.c.ID] = f.c
	if err := f.uc.Execute(ctx, f.c.ID); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(f.corrector.calls) != 2 {
		t.Errorf("Corrector 呼叫 %d 次，want 2（chunk 0 已完成應跳過）", len(f.corrector.calls))
	}
	if last := f.repo.progress[len(f.repo.progress)-1]; len(last.Chunks) != 3 {
		t.Errorf("最終 progress = %+v，want 3 段", last)
	}
	if f.repo.applied == nil || f.repo.appliedSegs[0].Text != "已校正0" {
		t.Errorf("progress 中的 patch 應被套用：%+v", f.repo.applied)
	}
}

func TestExecute_ContextIsPreviousChunkTailAfterPatches(t *testing.T) {
	f := newExecFixture(thirtyMinSegs())
	f.corrector.byFirst[0] = []correction.Patch{{I: 1, Text: "校正後1"}}
	if err := f.uc.Execute(ctx, f.c.ID); err != nil {
		t.Fatalf("err = %v", err)
	}
	second := f.corrector.calls[1]
	if len(second.Context) != 2 || second.Context[1].Text != "校正後1" {
		t.Errorf("第二段上文 = %+v，want 上一段結尾且已套用 patch", second.Context)
	}
	if len(f.corrector.calls[0].Context) != 0 {
		t.Error("第一段不應有上文")
	}
}

func TestExecute_AppliesValidatedPatchesAndMerges(t *testing.T) {
	segs := []meeting.TranscriptSegment{
		{Speaker: "A", Text: "小娃處理", StartMs: 0, EndMs: 1000},
		{Speaker: "B", Text: "人工改過", StartMs: 1000, EndMs: 2000, ManuallyEdited: true},
	}
	f := newExecFixture(segs)
	f.corrector.byFirst[0] = []correction.Patch{{I: 0, Text: "小王處理"}, {I: 1, Text: "鎖定段改寫"}}
	manual := actionitem.ActionItem{ID: uuid.New(), Description: "整理規格書", Source: "manual"}
	plain := actionitem.ActionItem{ID: uuid.New(), Description: "舊項", Source: "llm"}
	f.items.items = []actionitem.ActionItem{manual, plain}
	f.extractor.out = actionitem.Extraction{Summary: "新摘要", Items: []actionitem.Extracted{{Description: "整理規格書"}, {Description: "確認現金流"}}}

	if err := f.uc.Execute(ctx, f.c.ID); err != nil {
		t.Fatalf("err = %v", err)
	}
	r := f.repo.applied
	if r == nil {
		t.Fatal("未呼叫 ApplyResult")
	}
	segsOut := f.repo.appliedSegs
	if segsOut[0].Text != "小王處理" || segsOut[0].OriginalText != "小娃處理" {
		t.Errorf("seg0 = %+v", segsOut[0])
	}
	if segsOut[1].Text != "人工改過" || segsOut[1].OriginalText != "" {
		t.Errorf("鎖定段不應被改：%+v", segsOut[1])
	}
	if len(r.Patches) != 1 || r.Patches[0].I != 0 {
		t.Errorf("鎖定段的 patch 應在驗證時丟棄：%+v", r.Patches)
	}
	if r.Summary != "新摘要" || len(r.SummarySections) != 1 || r.SummarySections[0].Title != "新" {
		t.Errorf("summary = %q sections = %+v", r.Summary, r.SummarySections)
	}
	if len(r.DeleteItemIDs) != 1 || r.DeleteItemIDs[0] != plain.ID || len(r.InsertItems) != 1 ||
		r.InsertItems[0].Description != "確認現金流" || r.InsertSortFrom != 1 {
		t.Errorf("merge = del %v ins %+v from %d", r.DeleteItemIDs, r.InsertItems, r.InsertSortFrom)
	}
	if len(f.extractor.gotExisting) != 1 || f.extractor.gotExisting[0] != "整理規格書" {
		t.Errorf("existing = %v", f.extractor.gotExisting)
	}
	if f.indexer.called != 1 {
		t.Errorf("reindex = %d", f.indexer.called)
	}
	var cs []string
	for _, e := range f.notifier.events {
		cs = append(cs, e.CorrectionStatus)
	}
	if strings.Join(cs, ",") != "running,completed" {
		t.Errorf("events = %v", cs)
	}
	if f.corrector.calls[0].Segments[1].Locked != true {
		t.Error("ManuallyEdited 段應以 Locked 送給模型")
	}
}

func TestExecute_UserEditDuringCorrectionWins(t *testing.T) {
	segs := []meeting.TranscriptSegment{{Speaker: "A", Text: "小娃處理", StartMs: 0, EndMs: 1000}}
	f := newExecFixture(segs)
	edited := f.meetings.m
	edited.TranscriptSegments = []meeting.TranscriptSegment{{Speaker: "A", Text: "使用者剛改", StartMs: 0, EndMs: 1000, ManuallyEdited: true}}
	f.meetings.afterFirst = &edited
	f.corrector.byFirst[0] = []correction.Patch{{I: 0, Text: "小王處理"}}
	if err := f.uc.Execute(ctx, f.c.ID); err != nil {
		t.Fatalf("err = %v", err)
	}
	if got := f.repo.appliedSegs[0].Text; got != "使用者剛改" {
		t.Errorf("Text = %q，校正期間的人工編輯應優先", got)
	}
}

func TestExecute_CorrectorBadOutputTreatedAsNoChange(t *testing.T) {
	f := newExecFixture([]meeting.TranscriptSegment{{Speaker: "A", Text: "原文", StartMs: 0, EndMs: 1000}})
	f.corrector.err = fmt.Errorf("parse: %w", correction.ErrBadOutput)
	if err := f.uc.Execute(ctx, f.c.ID); err != nil {
		t.Fatalf("壞輸出不應讓整體失敗：%v", err)
	}
	if f.repo.applied == nil || len(f.repo.applied.Patches) != 0 {
		t.Errorf("applied = %+v", f.repo.applied)
	}
	if p := f.repo.progress[0]; len(p.Chunks["0"]) != 0 {
		t.Errorf("progress = %+v", p)
	}
}

func TestExecute_CorrectorErrorPropagatesForRetry(t *testing.T) {
	f := newExecFixture([]meeting.TranscriptSegment{{Speaker: "A", Text: "原文", StartMs: 0, EndMs: 1000}})
	f.corrector.err = errors.New("gemini 503")
	if err := f.uc.Execute(ctx, f.c.ID); err == nil {
		t.Fatal("外部錯誤應回傳讓佇列重試")
	}
	if f.repo.applied != nil {
		t.Error("失敗時不應套用")
	}
}

func TestMarkFailed_SetsFailedAndNotifies(t *testing.T) {
	f := newExecFixture(thirtyMinSegs())
	f.c.Status = correction.StatusRunning
	f.repo.stored[f.c.ID] = f.c
	f.uc.MarkFailed(ctx, f.c.ID, errors.New("boom secret detail"))
	got := f.repo.stored[f.c.ID]
	if got.Status != correction.StatusFailed || strings.Contains(got.ErrorMessage, "secret") || got.ErrorMessage == "" {
		t.Errorf("got %+v", got)
	}
	if len(f.notifier.events) != 1 || f.notifier.events[0].CorrectionStatus != "failed" {
		t.Errorf("events = %+v", f.notifier.events)
	}
}
