package meeting

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	appmeeting "github.com/as130232/busy-bee/busy-bee-be/application/meeting"
	domainactionitem "github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

type replaceRepo struct{ m domainmeeting.Meeting }

func (r *replaceRepo) GetForUser(context.Context, uuid.UUID, uuid.UUID) (domainmeeting.Meeting, error) {
	return r.m, nil
}
func (r *replaceRepo) UpdateTranscriptSegments(_ context.Context, _, _ uuid.UUID, segs []domainmeeting.TranscriptSegment, t string) (domainmeeting.Meeting, error) {
	r.m.TranscriptSegments, r.m.Transcript = segs, t
	return r.m, nil
}
func (r *replaceRepo) SaveSummary(_ context.Context, _, _ uuid.UUID, s string) (domainmeeting.Meeting, error) {
	r.m.Summary = s
	return r.m, nil
}
func (r *replaceRepo) SaveSummarySections(_ context.Context, _, _ uuid.UUID, s []domainmeeting.SummarySection) (domainmeeting.Meeting, error) {
	r.m.SummarySections = s
	return r.m, nil
}

type replaceItems struct{}

func (replaceItems) ListByMeeting(context.Context, uuid.UUID) ([]domainactionitem.ActionItem, error) {
	return []domainactionitem.ActionItem{{ID: uuid.New(), Description: "寄 DBA 報告"}}, nil
}
func (replaceItems) UpdateDescription(_ context.Context, id, _ uuid.UUID, d string) (domainactionitem.ActionItem, error) {
	return domainactionitem.ActionItem{ID: id, Description: d}, nil
}

func replaceRouter(repo *replaceRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewHandler(HandlerUCs{ReplaceText: appmeeting.NewReplaceTextUC(repo, replaceItems{}, nil)})
	e := gin.New()
	e.POST("/meetings/:id/transcript/replace", withTestUserID(), h.ReplaceText)
	return e
}

func TestReplaceText_ReturnsMeetingAndCounts(t *testing.T) {
	repo := &replaceRepo{m: domainmeeting.Meeting{
		ID: uuid.New(), Status: domainmeeting.StatusCompleted, Summary: "DBA",
		TranscriptSegments: []domainmeeting.TranscriptSegment{{Speaker: "A", Text: "DBA 報告"}},
	}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/meetings/"+repo.m.ID.String()+"/transcript/replace",
		strings.NewReader(`{"from":"DBA","to":"DVA","includeSummary":true}`))
	req.Header.Set("Content-Type", "application/json")
	replaceRouter(repo).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Meeting struct {
				Summary            string `json:"summary"`
				TranscriptSegments []struct {
					Text string `json:"text"`
				} `json:"transcriptSegments"`
			} `json:"meeting"`
			Counts struct {
				Transcript, Summary, ActionItems int
			} `json:"counts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data.Counts.Transcript != 1 || resp.Data.Counts.Summary != 1 || resp.Data.Counts.ActionItems != 1 {
		t.Errorf("counts = %+v", resp.Data.Counts)
	}
	if resp.Data.Meeting.Summary != "DVA" || resp.Data.Meeting.TranscriptSegments[0].Text != "DVA 報告" {
		t.Errorf("meeting = %+v", resp.Data.Meeting)
	}
}

func TestReplaceText_InvalidRule400(t *testing.T) {
	repo := &replaceRepo{m: domainmeeting.Meeting{ID: uuid.New(), Status: domainmeeting.StatusCompleted}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/meetings/"+repo.m.ID.String()+"/transcript/replace",
		strings.NewReader(`{"from":"DBA","to":""}`))
	req.Header.Set("Content-Type", "application/json")
	replaceRouter(repo).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
