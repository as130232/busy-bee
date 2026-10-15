// Package live 提供即時會議（F-LIVE）HTTP handlers：開始、STT 短效金鑰、用量心跳、即時重點、
// 會議中提問、結束，以及會前資料與使用者偏好。
package live

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	applive "github.com/as130232/busy-bee/busy-bee-be/application/live"
	appmc "github.com/as130232/busy-bee/busy-bee-be/application/meetingcontext"
	domainlive "github.com/as130232/busy-bee/busy-bee-be/domain/live"
	"github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainmc "github.com/as130232/busy-bee/busy-bee-be/domain/meetingcontext"
	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/response"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// LiveUC application/live.UseCases 的形狀。
type LiveUC interface {
	Start(ctx context.Context, userID uuid.UUID, in applive.StartInput) (meeting.Meeting, error)
	Token(ctx context.Context, userID, meetingID uuid.UUID) (applive.TokenOutput, error)
	Heartbeat(ctx context.Context, userID, meetingID uuid.UUID, seconds int) (applive.Usage, error)
	Notes(ctx context.Context, userID, meetingID uuid.UUID, in applive.NotesInput) (meeting.LiveSummary, error)
	Ask(ctx context.Context, userID, meetingID uuid.UUID, in applive.AskInput) (domainlive.Answer, error)
	Finish(ctx context.Context, userID, meetingID uuid.UUID, notes meeting.LiveNotes) (applive.FinishOutput, error)
}

// ContextUC application/meetingcontext.UseCases 的形狀。
type ContextUC interface {
	Add(ctx context.Context, userID, meetingID uuid.UUID, in appmc.AddInput) (domainmc.Context, error)
	List(ctx context.Context, userID, meetingID uuid.UUID) ([]domainmc.Context, error)
	Delete(ctx context.Context, userID, meetingID, contextID uuid.UUID) error
}

type Handler struct {
	live         LiveUC
	contexts     ContextUC
	prefs        domainuser.PreferenceWriter
	maxFileBytes int64
}

func NewHandler(live LiveUC, contexts ContextUC, prefs domainuser.PreferenceWriter, maxFileBytes int64) *Handler {
	return &Handler{live: live, contexts: contexts, prefs: prefs, maxFileBytes: maxFileBytes}
}

// Register 掛載路由（需已通過 Auth + ResolveUser）。
func (h *Handler) Register(r gin.IRoutes) {
	r.POST("/meetings/live", h.Start)
	r.POST("/meetings/:id/live/token", h.Token)
	r.POST("/meetings/:id/live/heartbeat", h.Heartbeat)
	r.POST("/meetings/:id/live/notes", h.Notes)
	r.POST("/meetings/:id/live/ask", h.Ask)
	r.POST("/meetings/:id/live/finish", h.Finish)
	r.GET("/meetings/:id/contexts", h.ListContexts)
	r.POST("/meetings/:id/contexts", h.AddContext)
	r.DELETE("/meetings/:id/contexts/:contextId", h.DeleteContext)
	r.PATCH("/users/me/preferences", h.UpdatePreferences)
}

// ---- DTO ----

type startRequest struct {
	Title              string `json:"title"`
	Scenario           string `json:"scenario"`
	Language           string `json:"language"`
	ContentType        string `json:"contentType"`
	ScheduledMeetingID string `json:"scheduledMeetingId"`
}

type summaryDTO struct {
	Current string   `json:"current"`
	Points  []string `json:"points"`
}

type sourceDTO struct {
	Type  string `json:"type"`
	Label string `json:"label"`
}

type qaDTO struct {
	Question string      `json:"question"`
	Answer   string      `json:"answer"`
	Sources  []sourceDTO `json:"sources"`
	AskedAt  time.Time   `json:"askedAt"`
}

type usageDTO struct {
	UsedMinutes  int  `json:"usedMinutes"`
	LimitMinutes int  `json:"limitMinutes"`
	OverLimit    bool `json:"overLimit"`
}

type contextDTO struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Source    string    `json:"source"`
	Chars     int       `json:"chars"`
	CreatedAt time.Time `json:"createdAt"`
}

func toSummary(d summaryDTO) meeting.LiveSummary {
	return meeting.LiveSummary{Current: d.Current, Points: d.Points}
}

func fromSummary(s meeting.LiveSummary) summaryDTO {
	pts := s.Points
	if pts == nil {
		pts = []string{}
	}
	return summaryDTO{Current: s.Current, Points: pts}
}

func toSources(in []sourceDTO) []meeting.LiveSource {
	out := make([]meeting.LiveSource, len(in))
	for i, s := range in {
		out[i] = meeting.LiveSource{Type: s.Type, Label: s.Label}
	}
	return out
}

func fromSources(in []meeting.LiveSource) []sourceDTO {
	out := make([]sourceDTO, len(in))
	for i, s := range in {
		out[i] = sourceDTO{Type: s.Type, Label: s.Label}
	}
	return out
}

func toQA(in []qaDTO) []meeting.LiveQA {
	out := make([]meeting.LiveQA, len(in))
	for i, q := range in {
		out[i] = meeting.LiveQA{Question: q.Question, Answer: q.Answer, Sources: toSources(q.Sources), AskedAt: q.AskedAt}
	}
	return out
}

func fromUsage(u applive.Usage) usageDTO {
	return usageDTO{UsedMinutes: u.UsedMinutes, LimitMinutes: u.LimitMinutes, OverLimit: u.OverLimit}
}

func toContextDTO(c domainmc.Context) contextDTO {
	return contextDTO{ID: c.ID.String(), Title: c.Title, Source: string(c.Source), Chars: utf8.RuneCountInString(c.Content), CreatedAt: c.CreatedAt}
}

// meetingDTO 開始/結束回傳的精簡紀錄（完整欄位由 GET /meetings/:id 取得）。
type meetingDTO struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Scenario string `json:"scenario"`
	Language string `json:"language"`
}

func toMeetingDTO(m meeting.Meeting) meetingDTO {
	return meetingDTO{ID: m.ID.String(), Title: m.Title, Status: string(m.Status), Scenario: string(m.Scenario), Language: string(m.Language)}
}

// ---- handlers ----

// Start POST /api/v1/meetings/live — 建立（或從排程開始）即時會議。
func (h *Handler) Start(c *gin.Context) {
	userID, ok := userFrom(c)
	if !ok {
		return
	}
	var req startRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "body"))
		return
	}
	in := applive.StartInput{Title: req.Title, Scenario: req.Scenario, Language: req.Language, ContentType: req.ContentType}
	if req.ScheduledMeetingID != "" {
		id, err := uuid.Parse(req.ScheduledMeetingID)
		if err != nil {
			response.Fail(c, apperr.Wrap(err, errcode.Param, "scheduledMeetingId"))
			return
		}
		in.ScheduledMeetingID = &id
	}
	m, err := h.live.Start(c.Request.Context(), userID, in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"meeting": toMeetingDTO(m)})
}

// Token POST /api/v1/meetings/:id/live/token — Deepgram 短效金鑰與串流參數。
func (h *Handler) Token(c *gin.Context) {
	userID, meetingID, ok := ids(c)
	if !ok {
		return
	}
	out, err := h.live.Token(c.Request.Context(), userID, meetingID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{
		"accessToken": out.Token.AccessToken, "expiresIn": out.Token.ExpiresIn,
		"model": out.Model, "language": out.Language, "keyterms": out.Keyterms, "usage": fromUsage(out.Usage),
	})
}

// Heartbeat POST /api/v1/meetings/:id/live/heartbeat — 回報即時辨識秒數，回傳本月用量。
func (h *Handler) Heartbeat(c *gin.Context) {
	userID, meetingID, ok := ids(c)
	if !ok {
		return
	}
	var req struct {
		Seconds int `json:"seconds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "body"))
		return
	}
	u, err := h.live.Heartbeat(c.Request.Context(), userID, meetingID, req.Seconds)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"usage": fromUsage(u)})
}

// Notes POST /api/v1/meetings/:id/live/notes — 更新即時重點。
func (h *Handler) Notes(c *gin.Context) {
	userID, meetingID, ok := ids(c)
	if !ok {
		return
	}
	var req struct {
		Notes         summaryDTO `json:"notes"`
		NewTranscript string     `json:"newTranscript"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "body"))
		return
	}
	out, err := h.live.Notes(c.Request.Context(), userID, meetingID, applive.NotesInput{Current: toSummary(req.Notes), NewTranscript: req.NewTranscript})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"notes": fromSummary(out)})
}

// Ask POST /api/v1/meetings/:id/live/ask — 會議中提問（依本場逐字稿與會前資料）。
func (h *Handler) Ask(c *gin.Context) {
	userID, meetingID, ok := ids(c)
	if !ok {
		return
	}
	var req struct {
		Question   string  `json:"question"`
		Transcript string  `json:"transcript"`
		History    []qaDTO `json:"history"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "body"))
		return
	}
	ans, err := h.live.Ask(c.Request.Context(), userID, meetingID, applive.AskInput{Question: req.Question, Transcript: req.Transcript, History: toQA(req.History)})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"answer": ans.Text, "sources": fromSources(ans.Sources)})
}

// Finish POST /api/v1/meetings/:id/live/finish — 存會議中筆記並取得音檔直傳 URL。
func (h *Handler) Finish(c *gin.Context) {
	userID, meetingID, ok := ids(c)
	if !ok {
		return
	}
	var req struct {
		Notes summaryDTO `json:"notes"`
		QA    []qaDTO    `json:"qa"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "body"))
		return
	}
	out, err := h.live.Finish(c.Request.Context(), userID, meetingID, meeting.LiveNotes{Notes: toSummary(req.Notes), QA: toQA(req.QA)})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{
		"meeting": toMeetingDTO(out.Meeting),
		"upload":  gin.H{"url": out.Upload.URL, "headers": out.Upload.Headers},
	})
}

// ListContexts GET /api/v1/meetings/:id/contexts — 會前資料清單（不含全文）。
func (h *Handler) ListContexts(c *gin.Context) {
	userID, meetingID, ok := ids(c)
	if !ok {
		return
	}
	cs, err := h.contexts.List(c.Request.Context(), userID, meetingID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	out := make([]contextDTO, len(cs))
	for i, x := range cs {
		out[i] = toContextDTO(x)
	}
	response.OK(c, gin.H{"contexts": out})
}

// AddContext POST /api/v1/meetings/:id/contexts — JSON {title, text} 貼上文字，或 multipart（file, title）上傳檔案。
func (h *Handler) AddContext(c *gin.Context) {
	userID, meetingID, ok := ids(c)
	if !ok {
		return
	}
	var in appmc.AddInput
	if strings.HasPrefix(c.ContentType(), "multipart/") {
		// 多留 64KB 給 multipart 邊界與欄位；超過上限直接回 413，不讀完整檔。
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxFileBytes+64*1024)
		fh, err := c.FormFile("file")
		if err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				response.Fail(c, apperr.New(errcode.ContextTooLarge))
				return
			}
			response.Fail(c, apperr.Wrap(err, errcode.Param, "file"))
			return
		}
		if fh.Size > h.maxFileBytes {
			response.Fail(c, apperr.New(errcode.ContextTooLarge))
			return
		}
		f, err := fh.Open()
		if err != nil {
			response.Fail(c, apperr.Wrap(err, errcode.Param, "file"))
			return
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			response.Fail(c, apperr.Wrap(err, errcode.Param, "file"))
			return
		}
		in = appmc.AddInput{Title: c.PostForm("title"), Filename: fh.Filename, Data: data}
	} else {
		var req struct {
			Title string `json:"title"`
			Text  string `json:"text"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, apperr.Wrap(err, errcode.Param, "body"))
			return
		}
		in = appmc.AddInput{Title: req.Title, Text: req.Text}
	}
	out, err := h.contexts.Add(c.Request.Context(), userID, meetingID, in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"context": toContextDTO(out)})
}

// DeleteContext DELETE /api/v1/meetings/:id/contexts/:contextId
func (h *Handler) DeleteContext(c *gin.Context) {
	userID, meetingID, ok := ids(c)
	if !ok {
		return
	}
	contextID, err := uuid.Parse(c.Param("contextId"))
	if err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "contextId"))
		return
	}
	if err := h.contexts.Delete(c.Request.Context(), userID, meetingID, contextID); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{})
}

// UpdatePreferences PATCH /api/v1/users/me/preferences — 目前只有「即時辨識我的麥克風」。
func (h *Handler) UpdatePreferences(c *gin.Context) {
	userID, ok := userFrom(c)
	if !ok {
		return
	}
	var req struct {
		LiveMicTranscription *bool `json:"liveMicTranscription"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.LiveMicTranscription == nil {
		response.Fail(c, apperr.New(errcode.Param, "liveMicTranscription"))
		return
	}
	u, err := h.prefs.SetLiveMicTranscription(c.Request.Context(), userID, *req.LiveMicTranscription)
	if err != nil {
		if errors.Is(err, domainuser.ErrNotFound) {
			response.Fail(c, apperr.New(errcode.NotFound))
			return
		}
		response.Fail(c, apperr.Wrap(err, errcode.Internal))
		return
	}
	response.OK(c, gin.H{"liveMicTranscription": u.LiveMicTranscription})
}

func userFrom(c *gin.Context) (uuid.UUID, bool) {
	userID, ok := domainuser.IDFrom(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.New(errcode.Unauthorized))
	}
	return userID, ok
}

func ids(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	userID, ok := userFrom(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	meetingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "id"))
		return uuid.Nil, uuid.Nil, false
	}
	return userID, meetingID, true
}
