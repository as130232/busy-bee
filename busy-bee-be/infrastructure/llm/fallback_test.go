package llm

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/genai"
)

// fakeGen 依模型名回傳結果，記錄呼叫順序與各次設定。
type fakeGen struct {
	errs   map[string]error
	called []string
	cfgs   []*genai.GenerateContentConfig
}

func (f *fakeGen) gen(_ context.Context, model string, _ []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	f.called = append(f.called, model)
	f.cfgs = append(f.cfgs, cfg)
	if err := f.errs[model]; err != nil {
		return nil, err
	}
	return &genai.GenerateContentResponse{Candidates: []*genai.Candidate{{Content: genai.NewContentFromText("ok "+model, genai.RoleModel)}}}, nil
}

func TestGenerateContent_FallsBackWhenModelNotFound(t *testing.T) {
	f := &fakeGen{errs: map[string]error{"gemini-3-flash-preview": genai.APIError{Code: 404, Status: "NOT_FOUND"}}}
	c := &GeminiClient{model: "gemini-3-flash-preview", fallback: "gemini-3.5-flash", gen: f.gen}
	resp, err := c.generateContent(context.Background(), "label", genai.Text("hi"), false)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if resp.Text() != "ok gemini-3.5-flash" || len(f.called) != 2 || f.called[1] != "gemini-3.5-flash" {
		t.Errorf("called = %v text = %q", f.called, resp.Text())
	}
	// 備援模型也套用其自身的生成設定（gemini-3* → thinking LOW）
	if f.cfgs[1] == nil || f.cfgs[1].ThinkingConfig == nil || f.cfgs[1].ThinkingConfig.ThinkingLevel != genai.ThinkingLevelLow {
		t.Errorf("fallback cfg = %+v", f.cfgs[1])
	}
}

func TestGenerateContent_OtherErrorsDoNotFallBack(t *testing.T) {
	f := &fakeGen{errs: map[string]error{"gemini-3-flash-preview": genai.APIError{Code: 503, Status: "UNAVAILABLE"}}}
	c := &GeminiClient{model: "gemini-3-flash-preview", fallback: "gemini-3.5-flash", gen: f.gen}
	if _, err := c.generateContent(context.Background(), "label", genai.Text("hi"), false); err == nil {
		t.Fatal("503 應回錯讓佇列重試，不換模型")
	}
	if len(f.called) != 1 {
		t.Errorf("called = %v", f.called)
	}
}

func TestGenerateContent_NoFallbackConfigured(t *testing.T) {
	f := &fakeGen{errs: map[string]error{"m": genai.APIError{Code: 404, Status: "NOT_FOUND"}}}
	c := &GeminiClient{model: "m", gen: f.gen}
	_, err := c.generateContent(context.Background(), "label", genai.Text("hi"), false)
	if !isModelNotFound(err) || len(f.called) != 1 {
		t.Errorf("err = %v called = %v", err, f.called)
	}
}

func TestGenerateContent_JSONMode(t *testing.T) {
	f := &fakeGen{}
	c := &GeminiClient{model: "gemini-flash-latest", gen: f.gen}
	if _, err := c.generateContent(context.Background(), "label", genai.Text("hi"), true); err != nil {
		t.Fatal(err)
	}
	if f.cfgs[0] == nil || f.cfgs[0].ResponseMIMEType != "application/json" {
		t.Errorf("cfg = %+v", f.cfgs[0])
	}
}

func TestIsModelNotFound(t *testing.T) {
	if !isModelNotFound(errors.Join(errors.New("wrap"), genai.APIError{Code: 404})) {
		t.Error("包裝過的 404 應判定為 not found")
	}
	if isModelNotFound(errors.New("plain")) {
		t.Error("一般錯誤不應判定為 not found")
	}
}
