package qa

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	appsearch "github.com/as130232/busy-bee/busy-bee-be/application/search"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainsearch "github.com/as130232/busy-bee/busy-bee-be/domain/search"
	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
)

type fakeEmbedder struct{}

func (fakeEmbedder) Embed(context.Context, string) ([]float32, error) { return []float32{0.1}, nil }

type fakeRetriever struct{ chunks []domainsearch.RetrievedChunk }

func (r fakeRetriever) SearchSimilarForQA(context.Context, uuid.UUID, []float32, int) ([]domainsearch.RetrievedChunk, error) {
	return r.chunks, nil
}

type fakeCatalog struct{ meetings []domainmeeting.Meeting }

func (c fakeCatalog) ListForUser(context.Context, uuid.UUID, string) ([]domainmeeting.Meeting, error) {
	return c.meetings, nil
}

type fakeAnswerer struct{}

func (fakeAnswerer) Answer(context.Context, string, []domainsearch.QASource, []domainsearch.MeetingBrief) (string, error) {
	return "定價採基本盤加人頭 [1]。", nil
}

var testUserID = uuid.New()

func testRouter(chunks []domainsearch.RetrievedChunk) *gin.Engine {
	gin.SetMode(gin.TestMode)
	uc := appsearch.NewQAUC(fakeEmbedder{}, fakeRetriever{chunks: chunks}, fakeAnswerer{}, fakeCatalog{})
	h := NewHandler(uc)
	e := gin.New()
	inject := func(c *gin.Context) {
		c.Request = c.Request.WithContext(domainuser.WithID(c.Request.Context(), testUserID))
	}
	e.POST("/meetings/qa", inject, h.Ask)
	return e
}

func post(e *gin.Engine, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/meetings/qa", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.ServeHTTP(w, req)
	return w
}

func TestAsk_ReturnsAnswerAndSources(t *testing.T) {
	e := testRouter([]domainsearch.RetrievedChunk{
		{MeetingID: uuid.New(), Title: "定價會議", Content: "採用基本盤加人頭計價", Score: 0.9},
	})
	w := post(e, `{"question":"定價策略是什麼？"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data answerResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Data.Answer == "" || resp.Data.NoMatch {
		t.Errorf("expected answer, got %+v", resp.Data)
	}
	if len(resp.Data.Sources) != 1 || resp.Data.Sources[0].Index != 1 {
		t.Errorf("expected 1 source with index 1, got %+v", resp.Data.Sources)
	}
}

func TestAsk_NoMatch_WhenNoRelevantChunks(t *testing.T) {
	e := testRouter(nil) // 檢索無結果
	w := post(e, `{"question":"無關問題"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"noMatch":true`) {
		t.Errorf("expected noMatch true, body = %s", w.Body.String())
	}
}

func TestAsk_EmptyQuestion400(t *testing.T) {
	e := testRouter(nil)
	w := post(e, `{"question":"   "}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAsk_InvalidBody400(t *testing.T) {
	e := testRouter(nil)
	w := post(e, `not-json`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
