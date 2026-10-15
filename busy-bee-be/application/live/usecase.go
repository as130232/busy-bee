// Package live 即時會議（F-LIVE）use cases：開始、簽發 STT 短效金鑰、用量心跳、即時重點、會議中提問、結束。
// 後端無狀態：即時逐字稿與目前重點由瀏覽器每次帶入（ADR-018）。
package live

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	domainlive "github.com/as130232/busy-bee/busy-bee-be/domain/live"
	"github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainmc "github.com/as130232/busy-bee/busy-bee-be/domain/meetingcontext"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

const (
	// maxAudioBytes 與一般上傳相同（F-UPLOAD 200MB）。
	maxAudioBytes = 200 * 1024 * 1024
	// maxHeartbeatSeconds 單次心跳最多計入的秒數（前端每 5 分鐘回報一次，留緩衝）。
	maxHeartbeatSeconds = 600
	maxNotePoints       = 15
	maxAskHistory       = 5
	maxQuestionChars    = 500
	maxStoredQA         = 50
	maxKeyterms         = 100
	notesCooldown       = 15 * time.Second
	askCooldown         = 5 * time.Second
)

// taipei 月用量以台灣時間切月；用固定時區避免容器缺 tzdata。
var taipei = time.FixedZone("UTC+8", 8*60*60)

// liveContentTypes 即時會議錄音允許的格式（瀏覽器 MediaRecorder 輸出）→ 副檔名。
var liveContentTypes = map[string]string{"audio/webm": ".webm", "audio/mp4": ".m4a"}

// MeetingReader 以 owner 條件讀取紀錄。
type MeetingReader interface {
	GetForUser(ctx context.Context, id, userID uuid.UUID) (meeting.Meeting, error)
}

// ContextLister 列出會前資料。
type ContextLister interface {
	List(ctx context.Context, meetingID, userID uuid.UUID) ([]domainmc.Context, error)
}

// UploadSigner 簽發音檔直傳 URL（domain/meeting.AudioStorage 子集）。
type UploadSigner interface {
	SignedUploadURL(ctx context.Context, objectPath, contentType string, maxBytes int64) (meeting.UploadTarget, error)
}

type Deps struct {
	Meetings  MeetingReader
	Repo      domainlive.Repository
	Contexts  ContextLister
	Storage   UploadSigner
	Tokens    domainlive.TokenIssuer
	Assistant domainlive.Assistant
	// Now 測試可替換；nil 時用 time.Now。
	Now func() time.Time
}

type Config struct {
	MonthlyMinutesLimit int
	// STTModel 串流辨識模型（如 nova-3）。
	STTModel string
	// Keyterms 全域術語（DEEPGRAM_KEYWORDS）。
	Keyterms      []string
	MaxNotesChars int
	MaxAskChars   int
}

// UseCases 即時會議 use cases；共用依賴與每場呼叫冷卻狀態（in-process，ADR-010 單 instance）。
type UseCases struct {
	d     Deps
	cfg   Config
	notes *cooldown
	ask   *cooldown
}

func New(d Deps, cfg Config) *UseCases {
	if d.Now == nil {
		d.Now = time.Now
	}
	return &UseCases{d: d, cfg: cfg, notes: newCooldown(notesCooldown), ask: newCooldown(askCooldown)}
}

// ---- Start ----

type StartInput struct {
	Title       string
	Scenario    string
	Language    string
	ContentType string
	// ScheduledMeetingID 從排程開始時帶入（沿用該排程的會前資料）。
	ScheduledMeetingID *uuid.UUID
}

func (uc *UseCases) Start(ctx context.Context, userID uuid.UUID, in StartInput) (meeting.Meeting, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return meeting.Meeting{}, apperr.New(errcode.Param, "title")
	}
	ext, ok := liveContentTypes[in.ContentType]
	if !ok {
		return meeting.Meeting{}, apperr.New(errcode.Param, "contentType")
	}
	path := fmt.Sprintf("audio/%s/%s%s", userID, uuid.New(), ext)

	if in.ScheduledMeetingID != nil {
		if _, err := uc.getMeeting(ctx, userID, *in.ScheduledMeetingID); err != nil {
			return meeting.Meeting{}, err
		}
		m, err := uc.d.Repo.StartLiveFromScheduled(ctx, *in.ScheduledMeetingID, userID, path)
		if errors.Is(err, meeting.ErrStatusConflict) {
			return meeting.Meeting{}, apperr.New(errcode.Conflict)
		}
		if err != nil {
			return meeting.Meeting{}, apperr.Wrap(err, errcode.Internal)
		}
		return m, nil
	}
	m, err := uc.d.Repo.CreateLive(ctx, meeting.Meeting{
		UserID: userID, Title: title, AudioGCSPath: path,
		Scenario: meeting.ParseScenario(in.Scenario), Language: meeting.ParseLanguage(in.Language),
	})
	if err != nil {
		return meeting.Meeting{}, apperr.Wrap(err, errcode.Internal)
	}
	return m, nil
}

// ---- Token ----

type TokenOutput struct {
	Token    domainlive.Token
	Model    string
	Language string
	Keyterms []string
	Usage    Usage
}

// Token 簽發瀏覽器直連 Deepgram 的短效金鑰與串流參數；超過月上限拒絕。
func (uc *UseCases) Token(ctx context.Context, userID, meetingID uuid.UUID) (TokenOutput, error) {
	m, err := uc.liveMeeting(ctx, userID, meetingID)
	if err != nil {
		return TokenOutput{}, err
	}
	usage, err := uc.usage(ctx, userID)
	if err != nil {
		return TokenOutput{}, err
	}
	if usage.OverLimit {
		return TokenOutput{}, apperr.New(errcode.LiveQuotaExceeded)
	}
	contexts, err := uc.d.Contexts.List(ctx, m.ID, userID)
	if err != nil {
		return TokenOutput{}, apperr.Wrap(err, errcode.Internal)
	}
	tok, err := uc.d.Tokens.Issue(ctx)
	if err != nil {
		return TokenOutput{}, apperr.Wrap(err, errcode.Internal)
	}
	return TokenOutput{
		Token: tok, Model: uc.cfg.STTModel, Language: streamLanguage(m.Language),
		Keyterms: domainmc.MergeKeyterms(uc.cfg.Keyterms, contexts, maxKeyterms), Usage: usage,
	}, nil
}

// streamLanguage 串流辨識語言。auto（detect_language）串流不支援，且 multi 對中文會壞（ADR-014），退回 zh-TW。
func streamLanguage(l meeting.Language) string {
	if l == meeting.LanguageEnUS {
		return string(meeting.LanguageEnUS)
	}
	return string(meeting.LanguageZhTW)
}

// ---- Usage / Heartbeat ----

type Usage struct {
	UsedMinutes  int
	LimitMinutes int
	OverLimit    bool
}

// Heartbeat 累加本場即時辨識秒數（clamp 0..maxHeartbeatSeconds），回傳本月用量。
func (uc *UseCases) Heartbeat(ctx context.Context, userID, meetingID uuid.UUID, seconds int) (Usage, error) {
	seconds = min(max(seconds, 0), maxHeartbeatSeconds)
	if _, err := uc.d.Repo.AddLiveSeconds(ctx, meetingID, userID, seconds); err != nil {
		if errors.Is(err, meeting.ErrStatusConflict) {
			return Usage{}, apperr.New(errcode.LiveNotActive)
		}
		return Usage{}, apperr.Wrap(err, errcode.Internal)
	}
	return uc.usage(ctx, userID)
}

func (uc *UseCases) usage(ctx context.Context, userID uuid.UUID) (Usage, error) {
	now := uc.d.Now().In(taipei)
	since := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, taipei)
	secs, err := uc.d.Repo.SumLiveSecondsSince(ctx, userID, since)
	if err != nil {
		return Usage{}, apperr.Wrap(err, errcode.Internal)
	}
	limit := uc.cfg.MonthlyMinutesLimit
	return Usage{UsedMinutes: int(secs / 60), LimitMinutes: limit, OverLimit: secs >= int64(limit)*60}, nil
}

// ---- Notes ----

type NotesInput struct {
	Current       meeting.LiveSummary
	NewTranscript string
}

// Notes 以「目前重點＋新逐字稿＋會前資料」更新即時重點。
func (uc *UseCases) Notes(ctx context.Context, userID, meetingID uuid.UUID, in NotesInput) (meeting.LiveSummary, error) {
	tr := strings.TrimSpace(in.NewTranscript)
	if tr == "" || utf8.RuneCountInString(tr) > uc.cfg.MaxNotesChars {
		return meeting.LiveSummary{}, apperr.New(errcode.Param, "newTranscript")
	}
	m, err := uc.liveMeeting(ctx, userID, meetingID)
	if err != nil {
		return meeting.LiveSummary{}, err
	}
	if !uc.notes.allow(m.ID, uc.d.Now()) {
		return meeting.LiveSummary{}, apperr.New(errcode.TooManyRequests)
	}
	bg, err := uc.background(ctx, m)
	if err != nil {
		return meeting.LiveSummary{}, err
	}
	out, err := uc.d.Assistant.UpdateNotes(ctx, domainlive.NotesInput{Background: bg, Current: in.Current, NewTranscript: tr})
	if err != nil {
		return meeting.LiveSummary{}, apperr.Wrap(err, errcode.Internal)
	}
	if len(out.Points) > maxNotePoints {
		out.Points = out.Points[:maxNotePoints]
	}
	return out, nil
}

// ---- Ask ----

type AskInput struct {
	Question   string
	Transcript string
	History    []meeting.LiveQA
}

// Ask 依本場即時逐字稿與會前資料回答（只帶最近 maxAskHistory 輪問答）。
func (uc *UseCases) Ask(ctx context.Context, userID, meetingID uuid.UUID, in AskInput) (domainlive.Answer, error) {
	q := strings.TrimSpace(in.Question)
	if q == "" || utf8.RuneCountInString(q) > maxQuestionChars {
		return domainlive.Answer{}, apperr.New(errcode.Param, "question")
	}
	if utf8.RuneCountInString(in.Transcript) > uc.cfg.MaxAskChars {
		return domainlive.Answer{}, apperr.New(errcode.Param, "transcript")
	}
	m, err := uc.liveMeeting(ctx, userID, meetingID)
	if err != nil {
		return domainlive.Answer{}, err
	}
	if !uc.ask.allow(m.ID, uc.d.Now()) {
		return domainlive.Answer{}, apperr.New(errcode.TooManyRequests)
	}
	bg, err := uc.background(ctx, m)
	if err != nil {
		return domainlive.Answer{}, err
	}
	hist := in.History
	if len(hist) > maxAskHistory {
		hist = hist[len(hist)-maxAskHistory:]
	}
	ans, err := uc.d.Assistant.Ask(ctx, domainlive.AskInput{Background: bg, Transcript: in.Transcript, Question: q, History: hist})
	if err != nil {
		return domainlive.Answer{}, apperr.Wrap(err, errcode.Internal)
	}
	return ans, nil
}

// ---- Finish ----

type FinishOutput struct {
	Meeting meeting.Meeting
	Upload  meeting.UploadTarget
}

// Finish 存會議中筆記、live → scheduled，回傳音檔直傳 URL；之後走既有 complete-upload。
// 冪等：已結束的即時會議（scheduled 且有 LiveStartedAt）重發上傳 URL，不覆寫筆記（IndexedDB 救回重傳用）。
func (uc *UseCases) Finish(ctx context.Context, userID, meetingID uuid.UUID, notes meeting.LiveNotes) (FinishOutput, error) {
	m, err := uc.getMeeting(ctx, userID, meetingID)
	if err != nil {
		return FinishOutput{}, err
	}
	switch {
	case m.Status == meeting.StatusLive:
		if len(notes.QA) > maxStoredQA {
			notes.QA = notes.QA[len(notes.QA)-maxStoredQA:]
		}
		if m, err = uc.d.Repo.FinishLive(ctx, m.ID, userID, notes); err != nil {
			if errors.Is(err, meeting.ErrStatusConflict) {
				return FinishOutput{}, apperr.New(errcode.Conflict)
			}
			return FinishOutput{}, apperr.Wrap(err, errcode.Internal)
		}
	case m.Status == meeting.StatusScheduled && m.LiveStartedAt != nil:
	default:
		return FinishOutput{}, apperr.New(errcode.LiveNotActive)
	}
	ct := contentTypeOf(m.AudioGCSPath)
	target, err := uc.d.Storage.SignedUploadURL(ctx, m.AudioGCSPath, ct, maxAudioBytes)
	if err != nil {
		return FinishOutput{}, apperr.Wrap(err, errcode.Internal)
	}
	return FinishOutput{Meeting: m, Upload: target}, nil
}

func contentTypeOf(path string) string {
	for ct, ext := range liveContentTypes {
		if strings.HasSuffix(path, ext) {
			return ct
		}
	}
	return "audio/webm"
}

// ---- helpers ----

func (uc *UseCases) getMeeting(ctx context.Context, userID, meetingID uuid.UUID) (meeting.Meeting, error) {
	m, err := uc.d.Meetings.GetForUser(ctx, meetingID, userID)
	if errors.Is(err, meeting.ErrNotFound) {
		return meeting.Meeting{}, apperr.New(errcode.NotFound)
	}
	if err != nil {
		return meeting.Meeting{}, apperr.Wrap(err, errcode.Internal)
	}
	return m, nil
}

func (uc *UseCases) liveMeeting(ctx context.Context, userID, meetingID uuid.UUID) (meeting.Meeting, error) {
	m, err := uc.getMeeting(ctx, userID, meetingID)
	if err != nil {
		return meeting.Meeting{}, err
	}
	if m.Status != meeting.StatusLive {
		return meeting.Meeting{}, apperr.New(errcode.LiveNotActive)
	}
	return m, nil
}

func (uc *UseCases) background(ctx context.Context, m meeting.Meeting) (string, error) {
	cs, err := uc.d.Contexts.List(ctx, m.ID, m.UserID)
	if err != nil {
		return "", apperr.Wrap(err, errcode.Internal)
	}
	return domainmc.PromptBlock(cs), nil
}

// cooldown 每場會議的最小呼叫間隔（防前端異常連打 Gemini）。
type cooldown struct {
	mu   sync.Mutex
	gap  time.Duration
	last map[uuid.UUID]time.Time
}

func newCooldown(gap time.Duration) *cooldown {
	return &cooldown{gap: gap, last: map[uuid.UUID]time.Time{}}
}

func (c *cooldown) allow(id uuid.UUID, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if t, ok := c.last[id]; ok && now.Sub(t) < c.gap {
		return false
	}
	if len(c.last) > 1000 { // 清掉過期項，避免長時間執行後 map 無限成長
		for k, t := range c.last {
			if now.Sub(t) >= c.gap {
				delete(c.last, k)
			}
		}
	}
	c.last[id] = now
	return true
}
