package live

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	applive "github.com/as130232/busy-bee/busy-bee-be/application/live"
	appmc "github.com/as130232/busy-bee/busy-bee-be/application/meetingcontext"
	domainlive "github.com/as130232/busy-bee/busy-bee-be/domain/live"
	"github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainmc "github.com/as130232/busy-bee/busy-bee-be/domain/meetingcontext"
	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

type fakeLive struct {
	startIn  applive.StartInput
	notesIn  applive.NotesInput
	askIn    applive.AskInput
	finishIn meeting.LiveNotes
	seconds  int
	err      error
}

func (f *fakeLive) Start(_ context.Context, _ uuid.UUID, in applive.StartInput) (meeting.Meeting, error) {
	f.startIn = in
	return meeting.Meeting{ID: uuid.New(), Title: in.Title, Status: meeting.StatusLive}, f.err
}
func (f *fakeLive) Token(context.Context, uuid.UUID, uuid.UUID) (applive.TokenOutput, error) {
	return applive.TokenOutput{Token: domainlive.Token{AccessToken: "jwt", ExpiresIn: 30}, Model: "nova-3", Language: "zh-TW",
		Keyterms: []string{"QA"}, Usage: applive.Usage{UsedMinutes: 3, LimitMinutes: 100}}, f.err
}
func (f *fakeLive) Heartbeat(_ context.Context, _, _ uuid.UUID, seconds int) (applive.Usage, error) {
	f.seconds = seconds
	return applive.Usage{UsedMinutes: 5, LimitMinutes: 100, OverLimit: true}, f.err
}
func (f *fakeLive) Notes(_ context.Context, _, _ uuid.UUID, in applive.NotesInput) (meeting.LiveSummary, error) {
	f.notesIn = in
	return meeting.LiveSummary{Current: "c", Points: []string{"p"}}, f.err
}
func (f *fakeLive) Ask(_ context.Context, _, _ uuid.UUID, in applive.AskInput) (domainlive.Answer, error) {
	f.askIn = in
	return domainlive.Answer{Text: "a", Sources: []meeting.LiveSource{{Type: "transcript", Label: "14:03"}}}, f.err
}
func (f *fakeLive) Finish(_ context.Context, _, _ uuid.UUID, notes meeting.LiveNotes) (applive.FinishOutput, error) {
	f.finishIn = notes
	return applive.FinishOutput{Meeting: meeting.Meeting{ID: uuid.New(), Status: meeting.StatusScheduled},
		Upload: meeting.UploadTarget{URL: "https://u", Headers: map[string]string{"Content-Type": "audio/webm"}}}, f.err
}

type fakeContexts struct {
	addIn appmc.AddInput
	delID uuid.UUID
	err   error
}

func (f *fakeContexts) Add(_ context.Context, _, meetingID uuid.UUID, in appmc.AddInput) (domainmc.Context, error) {
	f.addIn = in
	return domainmc.Context{ID: uuid.New(), MeetingID: meetingID, Title: "t", Source: domainmc.SourceFile, Content: "四個字元"}, f.err
}
func (f *fakeContexts) List(context.Context, uuid.UUID, uuid.UUID) ([]domainmc.Context, error) {
	return []domainmc.Context{{ID: uuid.New(), Title: "t", Source: domainmc.SourceText, Content: "ab"}}, f.err
}
func (f *fakeContexts) Delete(_ context.Context, _, _, contextID uuid.UUID) error {
	f.delID = contextID
	return f.err
}

type fakePrefs struct{ enabled bool }

func (f *fakePrefs) SetLiveMicTranscription(_ context.Context, userID uuid.UUID, enabled bool) (domainuser.User, error) {
	f.enabled = enabled
	return domainuser.User{ID: userID, LiveMicTranscription: enabled}, nil
}

func router(l *fakeLive, cs *fakeContexts, p *fakePrefs) *gin.Engine {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(domainuser.WithID(c.Request.Context(), uuid.New()))
	})
	NewHandler(l, cs, p, 1024).Register(e.Group(""))
	return e
}

func call(t *testing.T, e *gin.Engine, method, path, contentType string, body []byte) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	e.ServeHTTP(w, req)
	var env struct {
		Data map[string]any `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	return w, env.Data
}

func jsonBody(v any) []byte { b, _ := json.Marshal(v); return b }

func TestStart(t *testing.T) {
	l := &fakeLive{}
	sid := uuid.New()
	w, data := call(t, router(l, &fakeContexts{}, &fakePrefs{}), http.MethodPost, "/meetings/live", "application/json",
		jsonBody(map[string]any{"title": "週會", "scenario": "meeting", "language": "zh-TW", "contentType": "audio/webm", "scheduledMeetingId": sid.String()}))
	if w.Code != http.StatusOK || data["meeting"] == nil {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if l.startIn.Title != "週會" || l.startIn.ScheduledMeetingID == nil || *l.startIn.ScheduledMeetingID != sid {
		t.Errorf("start input = %+v", l.startIn)
	}
	w, _ = call(t, router(l, &fakeContexts{}, &fakePrefs{}), http.MethodPost, "/meetings/live", "application/json",
		jsonBody(map[string]any{"title": "x", "scheduledMeetingId": "bad"}))
	if w.Code != http.StatusBadRequest {
		t.Errorf("bad scheduledMeetingId status = %d", w.Code)
	}
}

func TestToken(t *testing.T) {
	w, data := call(t, router(&fakeLive{}, &fakeContexts{}, &fakePrefs{}), http.MethodPost, "/meetings/"+uuid.NewString()+"/live/token", "", nil)
	if w.Code != http.StatusOK || data["accessToken"] != "jwt" || data["model"] != "nova-3" || data["language"] != "zh-TW" {
		t.Fatalf("status %d data %v", w.Code, data)
	}
	if u := data["usage"].(map[string]any); u["usedMinutes"] != float64(3) || u["limitMinutes"] != float64(100) {
		t.Errorf("usage = %v", u)
	}
}

func TestErrorsPassThrough(t *testing.T) {
	l := &fakeLive{err: apperr.New(errcode.LiveQuotaExceeded)}
	w, _ := call(t, router(l, &fakeContexts{}, &fakePrefs{}), http.MethodPost, "/meetings/"+uuid.NewString()+"/live/token", "", nil)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d", w.Code)
	}
	w, _ = call(t, router(l, &fakeContexts{}, &fakePrefs{}), http.MethodPost, "/meetings/not-uuid/live/token", "", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("bad id status = %d", w.Code)
	}
}

func TestHeartbeatNotesAskFinish(t *testing.T) {
	l := &fakeLive{}
	e := router(l, &fakeContexts{}, &fakePrefs{})
	base := "/meetings/" + uuid.NewString() + "/live/"

	w, data := call(t, e, http.MethodPost, base+"heartbeat", "application/json", jsonBody(map[string]any{"seconds": 300}))
	if w.Code != 200 || l.seconds != 300 || data["usage"].(map[string]any)["overLimit"] != true {
		t.Errorf("heartbeat %d %v", w.Code, data)
	}

	w, data = call(t, e, http.MethodPost, base+"notes", "application/json",
		jsonBody(map[string]any{"notes": map[string]any{"current": "舊", "points": []string{"a"}}, "newTranscript": "[14:03] 對方: hi"}))
	if w.Code != 200 || l.notesIn.Current.Current != "舊" || l.notesIn.NewTranscript == "" {
		t.Errorf("notes %d in %+v", w.Code, l.notesIn)
	}
	if n := data["notes"].(map[string]any); n["current"] != "c" {
		t.Errorf("notes resp %v", data)
	}

	w, data = call(t, e, http.MethodPost, base+"ask", "application/json",
		jsonBody(map[string]any{"question": "q", "transcript": "t", "history": []map[string]any{{"question": "q0", "answer": "a0"}}}))
	if w.Code != 200 || data["answer"] != "a" || len(l.askIn.History) != 1 || l.askIn.History[0].Answer != "a0" {
		t.Errorf("ask %d %v in %+v", w.Code, data, l.askIn)
	}
	if src := data["sources"].([]any); len(src) != 1 {
		t.Errorf("sources %v", data["sources"])
	}

	w, data = call(t, e, http.MethodPost, base+"finish", "application/json",
		jsonBody(map[string]any{"notes": map[string]any{"current": "c", "points": []string{}}, "qa": []map[string]any{{"question": "q", "answer": "a", "sources": []any{}, "askedAt": "2026-10-15T14:00:00Z"}}}))
	if w.Code != 200 || data["upload"].(map[string]any)["url"] != "https://u" || len(l.finishIn.QA) != 1 || l.finishIn.Notes.Current != "c" {
		t.Errorf("finish %d %v in %+v", w.Code, data, l.finishIn)
	}
}

func TestContexts(t *testing.T) {
	cs := &fakeContexts{}
	e := router(&fakeLive{}, cs, &fakePrefs{})
	base := "/meetings/" + uuid.NewString() + "/contexts"

	w, data := call(t, e, http.MethodPost, base, "application/json", jsonBody(map[string]any{"title": "筆記", "text": "內容"}))
	if w.Code != 200 || cs.addIn.Text != "內容" || cs.addIn.Title != "筆記" {
		t.Fatalf("json add %d %+v", w.Code, cs.addIn)
	}
	if c := data["context"].(map[string]any); c["chars"] != float64(4) || c["content"] != nil {
		t.Errorf("context resp should have chars, not content: %v", c)
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "spec.pdf")
	_, _ = fw.Write([]byte("%PDF"))
	_ = mw.Close()
	w, _ = call(t, e, http.MethodPost, base, mw.FormDataContentType(), buf.Bytes())
	if w.Code != 200 || cs.addIn.Filename != "spec.pdf" || string(cs.addIn.Data) != "%PDF" {
		t.Fatalf("multipart add %d %s %+v", w.Code, w.Body.String(), cs.addIn)
	}

	buf.Reset()
	mw = multipart.NewWriter(&buf)
	fw, _ = mw.CreateFormFile("file", "big.txt")
	_, _ = fw.Write([]byte(strings.Repeat("x", 2048)))
	_ = mw.Close()
	w, _ = call(t, e, http.MethodPost, base, mw.FormDataContentType(), buf.Bytes())
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversize upload status = %d", w.Code)
	}

	w, data = call(t, e, http.MethodGet, base, "", nil)
	if w.Code != 200 || len(data["contexts"].([]any)) != 1 {
		t.Errorf("list %d %v", w.Code, data)
	}
	cid := uuid.New()
	w, _ = call(t, e, http.MethodDelete, base+"/"+cid.String(), "", nil)
	if w.Code != 200 || cs.delID != cid {
		t.Errorf("delete %d", w.Code)
	}
}

func TestPreferences(t *testing.T) {
	p := &fakePrefs{}
	w, data := call(t, router(&fakeLive{}, &fakeContexts{}, p), http.MethodPatch, "/users/me/preferences", "application/json",
		jsonBody(map[string]any{"liveMicTranscription": true}))
	if w.Code != 200 || !p.enabled || data["liveMicTranscription"] != true {
		t.Errorf("prefs %d %v", w.Code, data)
	}
}

// 前端偵測到持續無聲時隨心跳回報音訊診斷：記進 log（只有狀態欄位，字串截斷）。
func TestHeartbeatLogsAudioDiagnostics(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	l := &fakeLive{}
	e := router(l, &fakeContexts{}, &fakePrefs{})
	w, _ := call(t, e, http.MethodPost, "/meetings/"+uuid.NewString()+"/live/heartbeat", "application/json",
		jsonBody(map[string]any{"seconds": 15, "diagnostics": map[string]any{
			"reason": "no_audio", "recorderCtx": "suspended", "pcmCtx": "running", "pcmFrames": 0,
			"peakDb": -120, "tabTrack": "live/muted", "micTrack": strings.Repeat("x", 200), "micDenied": false,
		}}))
	if w.Code != 200 || l.seconds != 15 {
		t.Fatalf("heartbeat %d seconds=%d", w.Code, l.seconds)
	}
	out := buf.String()
	for _, want := range []string{`"msg":"live.audio_diagnostics"`, `"recorder_ctx":"suspended"`, `"tab_track":"live/muted"`, `"pcm_frames":0`} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %s: %s", want, out)
		}
	}
	if strings.Contains(out, strings.Repeat("x", 65)) {
		t.Error("diagnostic strings should be truncated")
	}
}

func TestHeartbeatWithoutDiagnosticsDoesNotLog(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	e := router(&fakeLive{}, &fakeContexts{}, &fakePrefs{})
	call(t, e, http.MethodPost, "/meetings/"+uuid.NewString()+"/live/heartbeat", "application/json", jsonBody(map[string]any{"seconds": 1}))
	if strings.Contains(buf.String(), "live.audio_diagnostics") {
		t.Error("should not log diagnostics when absent")
	}
}
