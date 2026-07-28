package search

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainsearch "github.com/as130232/busy-bee/busy-bee-be/domain/search"
)

type qaEmbedder struct {
	vec []float32
	err error
}

func (e qaEmbedder) Embed(context.Context, string) ([]float32, error) { return e.vec, e.err }

type qaRetriever struct {
	chunks []domainsearch.RetrievedChunk
	err    error
}

func (r qaRetriever) SearchSimilarForQA(context.Context, uuid.UUID, []float32, int) ([]domainsearch.RetrievedChunk, error) {
	return r.chunks, r.err
}

type qaCatalog struct {
	meetings []domainmeeting.Meeting
	err      error
}

func (c qaCatalog) ListForUser(context.Context, uuid.UUID, string) ([]domainmeeting.Meeting, error) {
	return c.meetings, c.err
}

// qaAnswerer 記錄是否被呼叫與收到的來源/清單，供驗證「無資料不呼叫 LLM」與編號正確。
type qaAnswerer struct {
	called   bool
	gotSrc   []domainsearch.QASource
	gotMeets []domainsearch.MeetingBrief
	answer   string
	err      error
}

func (a *qaAnswerer) Answer(_ context.Context, _ string, sources []domainsearch.QASource, meetings []domainsearch.MeetingBrief) (string, error) {
	a.called = true
	a.gotSrc = sources
	a.gotMeets = meetings
	return a.answer, a.err
}

func TestQA_HappyPath_ReturnsAnswerWithSources(t *testing.T) {
	m1, m2 := uuid.New(), uuid.New()
	retr := qaRetriever{chunks: []domainsearch.RetrievedChunk{
		{MeetingID: m1, Title: "定價會議", Content: "採用基本盤加人頭計價", Score: 0.9},
		{MeetingID: m2, Title: "產品會議", Content: "首年目標 300 付費用戶", Score: 0.72},
	}}
	ans := &qaAnswerer{answer: "定價採基本盤加人頭 [1]，首年目標 300 用戶 [2]。"}
	uc := NewQAUC(qaEmbedder{vec: []float32{0.1}}, retr, ans, qaCatalog{})

	res, err := uc.Execute(context.Background(), uuid.New(), "定價策略是什麼？")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.NoMatch {
		t.Fatal("NoMatch should be false")
	}
	if !ans.called {
		t.Fatal("answerer should be called")
	}
	if len(res.Sources) != 2 {
		t.Fatalf("sources = %d, want 2", len(res.Sources))
	}
	if res.Sources[0].Index != 1 || res.Sources[1].Index != 2 {
		t.Errorf("source indexes = %d,%d, want 1,2", res.Sources[0].Index, res.Sources[1].Index)
	}
	if res.Sources[0].MeetingID != m1 {
		t.Errorf("source[0] meetingID mismatch")
	}
}

func TestQA_BelowThresholdFiltered_Renumbered(t *testing.T) {
	keep, drop := uuid.New(), uuid.New()
	retr := qaRetriever{chunks: []domainsearch.RetrievedChunk{
		{MeetingID: drop, Title: "不相關", Content: "x", Score: 0.4}, // 低於門檻，濾除
		{MeetingID: keep, Title: "相關", Content: "y", Score: 0.8},
	}}
	ans := &qaAnswerer{answer: "答案 [1]"}
	uc := NewQAUC(qaEmbedder{vec: []float32{0.1}}, retr, ans, qaCatalog{})

	res, err := uc.Execute(context.Background(), uuid.New(), "問題")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(res.Sources) != 1 || res.Sources[0].MeetingID != keep {
		t.Fatalf("should keep only above-threshold source, got %+v", res.Sources)
	}
	if res.Sources[0].Index != 1 {
		t.Errorf("index should be renumbered to 1, got %d", res.Sources[0].Index)
	}
}

// 列舉/時間型問題：即使無內容片段命中，只要有會議清單就應呼叫 LLM（非 NoMatch）。
func TestQA_NoChunksButCatalog_CallsLLMWithMeetings(t *testing.T) {
	retr := qaRetriever{chunks: []domainsearch.RetrievedChunk{
		{MeetingID: uuid.New(), Title: "低分", Content: "x", Score: 0.3}, // 濾除
	}}
	ans := &qaAnswerer{answer: "本週有一場定價會議。"}
	cat := qaCatalog{meetings: []domainmeeting.Meeting{{ID: uuid.New(), Title: "定價會議"}}}
	uc := NewQAUC(qaEmbedder{vec: []float32{0.1}}, retr, ans, cat)

	res, err := uc.Execute(context.Background(), uuid.New(), "這週開了什麼會議")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.NoMatch {
		t.Error("NoMatch should be false when catalog is non-empty")
	}
	if !ans.called {
		t.Fatal("answerer should be called with meeting catalog")
	}
	if len(ans.gotSrc) != 0 {
		t.Errorf("sources should be empty, got %d", len(ans.gotSrc))
	}
	if len(ans.gotMeets) != 1 {
		t.Errorf("meetings passed to answerer = %d, want 1", len(ans.gotMeets))
	}
}

// 內容與清單皆空 → 不呼叫 LLM（成本護欄）。
func TestQA_NoChunksNoCatalog_SkipsLLM(t *testing.T) {
	retr := qaRetriever{chunks: []domainsearch.RetrievedChunk{
		{MeetingID: uuid.New(), Title: "低分", Content: "x", Score: 0.3},
	}}
	ans := &qaAnswerer{answer: "不該被呼叫"}
	uc := NewQAUC(qaEmbedder{vec: []float32{0.1}}, retr, ans, qaCatalog{})

	res, err := uc.Execute(context.Background(), uuid.New(), "問題")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !res.NoMatch {
		t.Error("NoMatch should be true")
	}
	if ans.called {
		t.Error("answerer must NOT be called when no chunks and no catalog (cost guard)")
	}
	if res.Answer != qaNoMatchMsg {
		t.Errorf("answer = %q, want no-match msg", res.Answer)
	}
}

// 會議清單載入失敗應降級（不擋內容問答），仍以內容片段作答。
func TestQA_CatalogError_DegradesToContent(t *testing.T) {
	retr := qaRetriever{chunks: []domainsearch.RetrievedChunk{
		{MeetingID: uuid.New(), Title: "t", Content: "c", Score: 0.9},
	}}
	ans := &qaAnswerer{answer: "答案 [1]"}
	uc := NewQAUC(qaEmbedder{vec: []float32{0.1}}, retr, ans, qaCatalog{err: errors.New("db down")})

	res, err := uc.Execute(context.Background(), uuid.New(), "問題")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.NoMatch || !ans.called {
		t.Fatal("should still answer from content when catalog fails")
	}
	if len(ans.gotMeets) != 0 {
		t.Errorf("meetings should be empty on catalog error, got %d", len(ans.gotMeets))
	}
}

func TestQA_EmbedError_Propagates(t *testing.T) {
	uc := NewQAUC(qaEmbedder{err: errors.New("boom")}, qaRetriever{}, &qaAnswerer{}, qaCatalog{})
	if _, err := uc.Execute(context.Background(), uuid.New(), "q"); err == nil {
		t.Fatal("expected error on embed failure")
	}
}

func TestQA_AnswererError_Propagates(t *testing.T) {
	retr := qaRetriever{chunks: []domainsearch.RetrievedChunk{
		{MeetingID: uuid.New(), Title: "t", Content: "c", Score: 0.9},
	}}
	uc := NewQAUC(qaEmbedder{vec: []float32{0.1}}, retr, &qaAnswerer{err: errors.New("llm down")}, qaCatalog{})
	if _, err := uc.Execute(context.Background(), uuid.New(), "q"); err == nil {
		t.Fatal("expected error on answerer failure")
	}
}
