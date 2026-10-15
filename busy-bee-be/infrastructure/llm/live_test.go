package llm

import (
	"context"
	"strings"
	"testing"
	"time"

	"google.golang.org/genai"

	domainlive "github.com/as130232/busy-bee/busy-bee-be/domain/live"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

// replyGen 回傳固定文字並記錄送出的 contents 與設定。
type replyGen struct {
	reply    string
	contents []*genai.Content
	cfg      *genai.GenerateContentConfig
}

func (r *replyGen) gen(_ context.Context, _ string, contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	r.contents, r.cfg = contents, cfg
	return &genai.GenerateContentResponse{Candidates: []*genai.Candidate{{Content: genai.NewContentFromText(r.reply, genai.RoleModel)}}}, nil
}

func (r *replyGen) promptText() string {
	var b strings.Builder
	for _, c := range r.contents {
		for _, p := range c.Parts {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

func TestUpdateNotes(t *testing.T) {
	r := &replyGen{reply: "```json\n{\"current\":\"討論部署\",\"points\":[\"週三上線\"]}\n```"}
	c := &GeminiClient{model: "gemini-3-flash-preview", gen: r.gen}
	out, err := c.UpdateNotes(context.Background(), domainlive.NotesInput{
		Background:    "## 背景資料（會前提供，非會議內容）\n### 規格\nX",
		Current:       domainmeeting.LiveSummary{Current: "開場", Points: []string{"舊重點"}},
		NewTranscript: "[14:03] 對方: 週三上線",
	})
	if err != nil || out.Current != "討論部署" || len(out.Points) != 1 {
		t.Fatalf("out = %+v err %v", out, err)
	}
	p := r.promptText()
	if !strings.HasPrefix(p, "## 背景資料") {
		t.Errorf("背景資料需放 prompt 最前面（隱性快取）：%q", p[:40])
	}
	for _, want := range []string{"[14:03] 對方: 週三上線", "目前在討論：開場", "- 舊重點"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Contains(p, "{{") {
		t.Error("placeholder not replaced")
	}
	if r.cfg == nil || r.cfg.ResponseMIMEType != "application/json" {
		t.Errorf("cfg = %+v, want JSON output", r.cfg)
	}
}

func TestUpdateNotes_EmptyStateAndBadOutput(t *testing.T) {
	r := &replyGen{reply: "not json"}
	c := &GeminiClient{model: "m", gen: r.gen}
	if _, err := c.UpdateNotes(context.Background(), domainlive.NotesInput{NewTranscript: "x"}); err == nil {
		t.Error("bad output must error")
	}
	if p := r.promptText(); !strings.Contains(p, "（尚無）") || strings.HasPrefix(p, "\n") {
		t.Errorf("empty current/background rendering: %q", p[:60])
	}
}

func TestAsk(t *testing.T) {
	r := &replyGen{reply: `{"answer":"10/22〔逐字稿 14:03〕","sources":[{"type":"transcript","label":"14:03"}]}`}
	c := &GeminiClient{model: "m", gen: r.gen}
	ans, err := c.Ask(context.Background(), domainlive.AskInput{
		Transcript: "[14:03] 對方: 10/22 上線",
		Question:   "上線日期？",
		History:    []domainmeeting.LiveQA{{Question: "誰負責？", Answer: "Ben", AskedAt: time.Now()}},
	})
	if err != nil || ans.Text != "10/22〔逐字稿 14:03〕" || len(ans.Sources) != 1 || ans.Sources[0].Label != "14:03" {
		t.Fatalf("ans = %+v err %v", ans, err)
	}
	p := r.promptText()
	for _, want := range []string{"問題：上線日期？", "Q: 誰負責？\nA: Ben", "[14:03] 對方: 10/22 上線"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestAsk_DropsUnknownSourceTypes(t *testing.T) {
	r := &replyGen{reply: `{"answer":"a","sources":[{"type":"web","label":"x"},{"type":"context","label":"規格"}]}`}
	c := &GeminiClient{model: "m", gen: r.gen}
	ans, err := c.Ask(context.Background(), domainlive.AskInput{Question: "q"})
	if err != nil || len(ans.Sources) != 1 || ans.Sources[0].Type != "context" {
		t.Fatalf("ans = %+v err %v", ans, err)
	}
}

func TestExtractContext(t *testing.T) {
	r := &replyGen{reply: `{"content":"# 規格","keyterms":["Stage"]}`}
	c := &GeminiClient{model: "m", gen: r.gen}
	out, err := c.ExtractContext(context.Background(), []byte("%PDF-1.4"), "application/pdf")
	if err != nil || out.Content != "# 規格" || len(out.Keyterms) != 1 {
		t.Fatalf("pdf out = %+v err %v", out, err)
	}
	parts := r.contents[0].Parts
	if len(parts) != 2 || parts[1].InlineData == nil || parts[1].InlineData.MIMEType != "application/pdf" {
		t.Errorf("pdf must be sent as inline data part, got %+v", parts)
	}

	r.reply = `{"keyterms":["Jenkins"]}`
	out, err = c.ExtractContext(context.Background(), []byte("用 Jenkins 部署"), "text/plain")
	if err != nil || out.Content != "" || out.Keyterms[0] != "Jenkins" {
		t.Fatalf("text out = %+v err %v", out, err)
	}
	if !strings.Contains(r.promptText(), "用 Jenkins 部署") {
		t.Error("text not in prompt")
	}
	if _, err := c.ExtractContext(context.Background(), []byte("x"), "image/png"); err == nil {
		t.Error("unsupported mime must error")
	}
}

// 會前資料帶入會後流程：背景資料放 prompt 最前面並附「只用來理解」規則；無背景資料時 prompt 與原本完全相同。
func TestPostMeetingPrompts_Background(t *testing.T) {
	bg := "## 背景資料（會前提供，非會議內容）\n\n### 規格\nStage 驗證"
	r := &replyGen{reply: `{"sections":[],"tags":[],"summary":"","actionItems":[]}`}
	c := &GeminiClient{model: "m", gen: r.gen}

	if _, err := c.Summarize(context.Background(), "A: 逐字稿", domainmeeting.ScenarioMeeting, bg); err != nil {
		t.Fatal(err)
	}
	p := r.promptText()
	if !strings.HasPrefix(p, bg) || !strings.Contains(p, backgroundRule) {
		t.Errorf("summary prompt 背景資料需在最前面並附規則：%q", p[:80])
	}

	if _, err := c.Summarize(context.Background(), "A: 逐字稿", domainmeeting.ScenarioMeeting, ""); err != nil {
		t.Fatal(err)
	}
	want, _ := buildPrompt(scenarioPrompts[domainmeeting.ScenarioMeeting], "A: 逐字稿")
	if r.promptText() != want {
		t.Error("無背景資料時 summary prompt 應與原本完全相同")
	}

	if _, err := c.ExtractExcluding(context.Background(), "A: 逐字稿", time.Now(), nil, bg); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(r.promptText(), bg) {
		t.Error("action items prompt 背景資料需在最前面")
	}
}
