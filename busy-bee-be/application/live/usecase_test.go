package live

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	domainlive "github.com/as130232/busy-bee/busy-bee-be/domain/live"
	"github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainmc "github.com/as130232/busy-bee/busy-bee-be/domain/meetingcontext"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// ---- fakes ----

type fakeRepo struct {
	meetings  map[uuid.UUID]meeting.Meeting
	usedSecs  int64
	sinceArg  time.Time
	finishErr error
}

func newFakeRepo() *fakeRepo { return &fakeRepo{meetings: map[uuid.UUID]meeting.Meeting{}} }

func (r *fakeRepo) put(m meeting.Meeting) meeting.Meeting {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	r.meetings[m.ID] = m
	return m
}

func (r *fakeRepo) GetForUser(_ context.Context, id, userID uuid.UUID) (meeting.Meeting, error) {
	m, ok := r.meetings[id]
	if !ok || m.UserID != userID {
		return meeting.Meeting{}, meeting.ErrNotFound
	}
	return m, nil
}

func (r *fakeRepo) CreateLive(_ context.Context, m meeting.Meeting) (meeting.Meeting, error) {
	m.Status = meeting.StatusLive
	now := time.Now()
	m.LiveStartedAt = &now
	return r.put(m), nil
}

func (r *fakeRepo) StartLiveFromScheduled(_ context.Context, id, userID uuid.UUID, audioPath string) (meeting.Meeting, error) {
	m, ok := r.meetings[id]
	if !ok || m.UserID != userID || m.Status != meeting.StatusScheduled {
		return meeting.Meeting{}, meeting.ErrStatusConflict
	}
	m.Status, m.AudioGCSPath = meeting.StatusLive, audioPath
	return r.put(m), nil
}

func (r *fakeRepo) AddLiveSeconds(_ context.Context, id, userID uuid.UUID, seconds int) (meeting.Meeting, error) {
	m, ok := r.meetings[id]
	if !ok || m.UserID != userID || m.Status != meeting.StatusLive {
		return meeting.Meeting{}, meeting.ErrStatusConflict
	}
	m.LiveSeconds += seconds
	r.usedSecs += int64(seconds)
	return r.put(m), nil
}

func (r *fakeRepo) SumLiveSecondsSince(_ context.Context, _ uuid.UUID, since time.Time) (int64, error) {
	r.sinceArg = since
	return r.usedSecs, nil
}

func (r *fakeRepo) FinishLive(_ context.Context, id, userID uuid.UUID, notes meeting.LiveNotes) (meeting.Meeting, error) {
	if r.finishErr != nil {
		return meeting.Meeting{}, r.finishErr
	}
	m, ok := r.meetings[id]
	if !ok || m.UserID != userID || m.Status != meeting.StatusLive {
		return meeting.Meeting{}, meeting.ErrStatusConflict
	}
	m.Status, m.LiveNotes = meeting.StatusScheduled, &notes
	return r.put(m), nil
}

type fakeContexts struct{ list []domainmc.Context }

func (f *fakeContexts) List(context.Context, uuid.UUID, uuid.UUID) ([]domainmc.Context, error) {
	return f.list, nil
}

type fakeSigner struct{ path, contentType string }

func (f *fakeSigner) SignedUploadURL(_ context.Context, objectPath, contentType string, _ int64) (meeting.UploadTarget, error) {
	f.path, f.contentType = objectPath, contentType
	return meeting.UploadTarget{URL: "https://upload/" + objectPath}, nil
}

type fakeTokens struct{ calls int }

func (f *fakeTokens) Issue(context.Context) (domainlive.Token, error) {
	f.calls++
	return domainlive.Token{AccessToken: "jwt", ExpiresIn: 30}, nil
}

type fakeAssistant struct {
	notesIn  domainlive.NotesInput
	askIn    domainlive.AskInput
	notesOut meeting.LiveSummary
	err      error
}

func (f *fakeAssistant) UpdateNotes(_ context.Context, in domainlive.NotesInput) (meeting.LiveSummary, error) {
	f.notesIn = in
	return f.notesOut, f.err
}

func (f *fakeAssistant) Ask(_ context.Context, in domainlive.AskInput) (domainlive.Answer, error) {
	f.askIn = in
	return domainlive.Answer{Text: "答", Sources: []meeting.LiveSource{{Type: "transcript", Label: "14:03"}}}, f.err
}

type env struct {
	uc     *UseCases
	repo   *fakeRepo
	ctxs   *fakeContexts
	signer *fakeSigner
	tokens *fakeTokens
	ai     *fakeAssistant
	now    time.Time
	user   uuid.UUID
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{repo: newFakeRepo(), ctxs: &fakeContexts{}, signer: &fakeSigner{}, tokens: &fakeTokens{}, ai: &fakeAssistant{},
		now: time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC), user: uuid.New()}
	e.uc = New(Deps{
		Meetings: e.repo, Repo: e.repo, Contexts: e.ctxs, Storage: e.signer, Tokens: e.tokens, Assistant: e.ai,
		Now: func() time.Time { return e.now },
	}, Config{MonthlyMinutesLimit: 100, STTModel: "nova-3", Keyterms: []string{"QA"}, MaxNotesChars: 50, MaxAskChars: 80})
	return e
}

func (e *env) liveMeeting() meeting.Meeting {
	return e.repo.put(meeting.Meeting{UserID: e.user, Status: meeting.StatusLive, Language: meeting.LanguageZhTW, AudioGCSPath: "audio/u/x.webm"})
}

func wantCode(t *testing.T, err error, code errcode.ErrCode) {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != code {
		t.Fatalf("err = %v, want code %d", err, code)
	}
}

// ---- Start ----

func TestStart_CreatesLiveMeeting(t *testing.T) {
	e := newEnv(t)
	m, err := e.uc.Start(context.Background(), e.user, StartInput{Title: "  週會 ", Scenario: "casual", Language: "en-US", ContentType: "audio/webm"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != meeting.StatusLive || m.Title != "週會" || m.Scenario != meeting.ScenarioCasual || m.Language != meeting.LanguageEnUS {
		t.Errorf("meeting = %+v", m)
	}
	if !strings.HasPrefix(m.AudioGCSPath, "audio/"+e.user.String()+"/") || !strings.HasSuffix(m.AudioGCSPath, ".webm") {
		t.Errorf("audio path = %q", m.AudioGCSPath)
	}
}

func TestStart_Validation(t *testing.T) {
	e := newEnv(t)
	_, err := e.uc.Start(context.Background(), e.user, StartInput{Title: " ", ContentType: "audio/webm"})
	wantCode(t, err, errcode.Param)
	_, err = e.uc.Start(context.Background(), e.user, StartInput{Title: "t", ContentType: "audio/wav"})
	wantCode(t, err, errcode.Param)
}

func TestStart_FromScheduled(t *testing.T) {
	e := newEnv(t)
	s := e.repo.put(meeting.Meeting{UserID: e.user, Status: meeting.StatusScheduled, Title: "排程"})
	m, err := e.uc.Start(context.Background(), e.user, StartInput{Title: "t", ContentType: "audio/mp4", ScheduledMeetingID: &s.ID})
	if err != nil || m.ID != s.ID || m.Status != meeting.StatusLive || !strings.HasSuffix(m.AudioGCSPath, ".m4a") {
		t.Fatalf("m = %+v err %v", m, err)
	}
	other := uuid.New()
	_, err = e.uc.Start(context.Background(), other, StartInput{Title: "t", ContentType: "audio/webm", ScheduledMeetingID: &s.ID})
	wantCode(t, err, errcode.NotFound)
	_, err = e.uc.Start(context.Background(), e.user, StartInput{Title: "t", ContentType: "audio/webm", ScheduledMeetingID: &s.ID})
	wantCode(t, err, errcode.Conflict) // 已是 live
}

// ---- Token / usage ----

func TestToken_RequiresLiveAndQuota(t *testing.T) {
	e := newEnv(t)
	done := e.repo.put(meeting.Meeting{UserID: e.user, Status: meeting.StatusCompleted})
	_, err := e.uc.Token(context.Background(), e.user, done.ID)
	wantCode(t, err, errcode.LiveNotActive)

	m := e.liveMeeting()
	e.repo.usedSecs = 100 * 60 // 剛好用滿
	_, err = e.uc.Token(context.Background(), e.user, m.ID)
	wantCode(t, err, errcode.LiveQuotaExceeded)
	if e.tokens.calls != 0 {
		t.Error("超過上限不應簽發金鑰")
	}
}

func TestToken_ReturnsParams(t *testing.T) {
	e := newEnv(t)
	m := e.repo.put(meeting.Meeting{UserID: e.user, Status: meeting.StatusLive, Language: meeting.LanguageAuto})
	e.ctxs.list = []domainmc.Context{{Keyterms: []string{"Stage", "qa"}}}
	out, err := e.uc.Token(context.Background(), e.user, m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if out.Token.AccessToken != "jwt" || out.Model != "nova-3" || out.Language != "zh-TW" {
		t.Errorf("out = %+v（auto 串流退回 zh-TW）", out)
	}
	if !slices.Equal(out.Keyterms, []string{"QA", "Stage"}) {
		t.Errorf("keyterms = %v", out.Keyterms)
	}
}

func TestUsage_MonthStartIsTaipei(t *testing.T) {
	e := newEnv(t)
	e.now = time.Date(2026, 10, 31, 17, 0, 0, 0, time.UTC) // 台北 11/1 01:00
	m := e.liveMeeting()
	if _, err := e.uc.Heartbeat(context.Background(), e.user, m.ID, 60); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 11, 1, 0, 0, 0, 0, taipei)
	if !e.repo.sinceArg.Equal(want) {
		t.Errorf("since = %v, want %v", e.repo.sinceArg, want)
	}
}

func TestHeartbeat_ClampsAndReportsUsage(t *testing.T) {
	e := newEnv(t)
	m := e.liveMeeting()
	u, err := e.uc.Heartbeat(context.Background(), e.user, m.ID, 99999)
	if err != nil {
		t.Fatal(err)
	}
	if e.repo.meetings[m.ID].LiveSeconds != maxHeartbeatSeconds {
		t.Errorf("seconds not clamped: %d", e.repo.meetings[m.ID].LiveSeconds)
	}
	if u.UsedMinutes != 10 || u.LimitMinutes != 100 || u.OverLimit {
		t.Errorf("usage = %+v", u)
	}
	e.repo.usedSecs = 100*60 - 30
	if u, _ = e.uc.Heartbeat(context.Background(), e.user, m.ID, 30); !u.OverLimit {
		t.Errorf("用滿應 overLimit：%+v", u)
	}
	if _, err := e.uc.Heartbeat(context.Background(), e.user, m.ID, -5); err != nil {
		t.Errorf("負數應視為 0，不報錯：%v", err)
	}
}

// ---- Notes ----

func TestNotes(t *testing.T) {
	e := newEnv(t)
	m := e.liveMeeting()
	e.ctxs.list = []domainmc.Context{{Title: "規格", Content: "背景"}}
	pts := make([]string, 20)
	for i := range pts {
		pts[i] = "p"
	}
	e.ai.notesOut = meeting.LiveSummary{Current: "c", Points: pts}

	_, err := e.uc.Notes(context.Background(), e.user, m.ID, NotesInput{NewTranscript: "  "})
	wantCode(t, err, errcode.Param)
	_, err = e.uc.Notes(context.Background(), e.user, m.ID, NotesInput{NewTranscript: strings.Repeat("字", 51)})
	wantCode(t, err, errcode.Param)

	out, err := e.uc.Notes(context.Background(), e.user, m.ID, NotesInput{NewTranscript: "新內容", Current: meeting.LiveSummary{Current: "舊"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Points) != maxNotePoints {
		t.Errorf("points = %d, want capped %d", len(out.Points), maxNotePoints)
	}
	if !strings.Contains(e.ai.notesIn.Background, "### 規格") || e.ai.notesIn.Current.Current != "舊" {
		t.Errorf("assistant input = %+v", e.ai.notesIn)
	}

	_, err = e.uc.Notes(context.Background(), e.user, m.ID, NotesInput{NewTranscript: "x"})
	wantCode(t, err, errcode.TooManyRequests)
	e.now = e.now.Add(notesCooldown)
	if _, err = e.uc.Notes(context.Background(), e.user, m.ID, NotesInput{NewTranscript: "x"}); err != nil {
		t.Errorf("冷卻後應可再呼叫：%v", err)
	}
}

func TestNotes_AssistantErrorIsInternal(t *testing.T) {
	e := newEnv(t)
	m := e.liveMeeting()
	e.ai.err = errors.New("gemini down")
	_, err := e.uc.Notes(context.Background(), e.user, m.ID, NotesInput{NewTranscript: "x"})
	wantCode(t, err, errcode.Internal)
}

// ---- Ask ----

func TestAsk(t *testing.T) {
	e := newEnv(t)
	m := e.liveMeeting()
	_, err := e.uc.Ask(context.Background(), e.user, m.ID, AskInput{Question: " ", Transcript: "t"})
	wantCode(t, err, errcode.Param)
	_, err = e.uc.Ask(context.Background(), e.user, m.ID, AskInput{Question: "q", Transcript: strings.Repeat("字", 81)})
	wantCode(t, err, errcode.Param)

	hist := make([]meeting.LiveQA, 8)
	for i := range hist {
		hist[i] = meeting.LiveQA{Question: string(rune('a' + i))}
	}
	ans, err := e.uc.Ask(context.Background(), e.user, m.ID, AskInput{Question: " 上線日期？ ", Transcript: "t", History: hist})
	if err != nil || ans.Text != "答" {
		t.Fatalf("ans = %+v err %v", ans, err)
	}
	if e.ai.askIn.Question != "上線日期？" || len(e.ai.askIn.History) != maxAskHistory || e.ai.askIn.History[0].Question != "d" {
		t.Errorf("ask input = %+v（只帶最近 %d 輪）", e.ai.askIn, maxAskHistory)
	}
	_, err = e.uc.Ask(context.Background(), e.user, m.ID, AskInput{Question: "q", Transcript: "t"})
	wantCode(t, err, errcode.TooManyRequests)
}

// ---- Finish ----

func TestFinish(t *testing.T) {
	e := newEnv(t)
	m := e.liveMeeting()
	qa := make([]meeting.LiveQA, 60)
	out, err := e.uc.Finish(context.Background(), e.user, m.ID, meeting.LiveNotes{QA: qa})
	if err != nil {
		t.Fatal(err)
	}
	if out.Meeting.Status != meeting.StatusScheduled || out.Upload.URL == "" {
		t.Errorf("out = %+v", out)
	}
	if e.signer.path != m.AudioGCSPath || e.signer.contentType != "audio/webm" {
		t.Errorf("signed %q %q", e.signer.path, e.signer.contentType)
	}
	if got := len(e.repo.meetings[m.ID].LiveNotes.QA); got != maxStoredQA {
		t.Errorf("stored qa = %d, want %d", got, maxStoredQA)
	}

	// 冪等：已結束（scheduled + 有 LiveStartedAt）再呼叫 → 重發上傳 URL
	started := time.Now()
	fin := e.repo.put(meeting.Meeting{UserID: e.user, Status: meeting.StatusScheduled, LiveStartedAt: &started, AudioGCSPath: "audio/u/y.m4a"})
	if out, err = e.uc.Finish(context.Background(), e.user, fin.ID, meeting.LiveNotes{}); err != nil || out.Upload.URL == "" || e.signer.contentType != "audio/mp4" {
		t.Errorf("idempotent finish out=%+v err=%v ct=%q", out, err, e.signer.contentType)
	}

	plain := e.repo.put(meeting.Meeting{UserID: e.user, Status: meeting.StatusScheduled, AudioGCSPath: "a.webm"})
	_, err = e.uc.Finish(context.Background(), e.user, plain.ID, meeting.LiveNotes{})
	wantCode(t, err, errcode.LiveNotActive)
}
