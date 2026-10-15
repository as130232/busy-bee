package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"

	domainlive "github.com/as130232/busy-bee/busy-bee-be/domain/live"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainmc "github.com/as130232/busy-bee/busy-bee-be/domain/meetingcontext"
)

const (
	promptLiveNotes       = "prompts/live_notes.md"
	promptLiveAsk         = "prompts/live_ask.md"
	promptContextExtract  = "prompts/context_extract.md"
	promptContextKeyterms = "prompts/context_keyterms.md"
)

var (
	_ domainlive.Assistant = (*GeminiClient)(nil)
	_ domainmc.Extractor   = (*GeminiClient)(nil)
)

// renderTemplate 讀模板並依序替換 {{KEY}}。背景資料放在模板最前面：同一場會議重複呼叫時前綴相同，
// 讓 Gemini 隱性快取（implicit caching）命中，降低輸入成本；無背景資料時去掉開頭空行。
func renderTemplate(path string, vars map[string]string) (string, error) {
	raw, err := promptFS.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("llm.renderTemplate read %s: %w", path, err)
	}
	s := string(raw)
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}
	return strings.TrimLeft(s, "\n"), nil
}

// UpdateNotes 依目前重點與新增即時逐字稿更新會議中重點（F-LIVE）。
func (c *GeminiClient) UpdateNotes(ctx context.Context, in domainlive.NotesInput) (domainmeeting.LiveSummary, error) {
	prompt, err := renderTemplate(promptLiveNotes, map[string]string{
		"BACKGROUND": in.Background,
		"CURRENT":    currentNotesBlock(in.Current),
		"TRANSCRIPT": in.NewTranscript,
	})
	if err != nil {
		return domainmeeting.LiveSummary{}, err
	}
	text, err := c.completeJSON(ctx, promptLiveNotes, prompt)
	if err != nil {
		return domainmeeting.LiveSummary{}, err
	}
	var out domainmeeting.LiveSummary
	if err := json.Unmarshal([]byte(stripJSONFence(text)), &out); err != nil {
		return domainmeeting.LiveSummary{}, fmt.Errorf("llm.UpdateNotes parse: %w", err)
	}
	if out.Points == nil {
		out.Points = []string{}
	}
	return out, nil
}

func currentNotesBlock(s domainmeeting.LiveSummary) string {
	if strings.TrimSpace(s.Current) == "" && len(s.Points) == 0 {
		return "（尚無）"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "目前在討論：%s\n", s.Current)
	for _, p := range s.Points {
		fmt.Fprintf(&b, "- %s\n", p)
	}
	return strings.TrimSpace(b.String())
}

// Ask 依本場即時逐字稿與背景資料回答會議中提問，回傳答案與引用依據。
func (c *GeminiClient) Ask(ctx context.Context, in domainlive.AskInput) (domainlive.Answer, error) {
	var hist strings.Builder
	for _, h := range in.History {
		fmt.Fprintf(&hist, "Q: %s\nA: %s\n\n", h.Question, h.Answer)
	}
	prompt, err := renderTemplate(promptLiveAsk, map[string]string{
		"BACKGROUND": in.Background,
		"HISTORY":    orNone(hist.String()),
		"TRANSCRIPT": orNone(in.Transcript),
		"QUESTION":   in.Question,
	})
	if err != nil {
		return domainlive.Answer{}, err
	}
	text, err := c.completeJSON(ctx, promptLiveAsk, prompt)
	if err != nil {
		return domainlive.Answer{}, err
	}
	var out struct {
		Answer  string                     `json:"answer"`
		Sources []domainmeeting.LiveSource `json:"sources"`
	}
	if err := json.Unmarshal([]byte(stripJSONFence(text)), &out); err != nil {
		return domainlive.Answer{}, fmt.Errorf("llm.Ask parse: %w", err)
	}
	sources := []domainmeeting.LiveSource{}
	for _, s := range out.Sources {
		if (s.Type == "transcript" || s.Type == "context") && strings.TrimSpace(s.Label) != "" {
			sources = append(sources, s)
		}
	}
	return domainlive.Answer{Text: strings.TrimSpace(out.Answer), Sources: sources}, nil
}

// ExtractContext 會前資料轉文字：PDF 以 inline data 交給 Gemini 轉 markdown 並抽術語；text/plain 只抽術語。
func (c *GeminiClient) ExtractContext(ctx context.Context, data []byte, mimeType string) (domainmc.Extraction, error) {
	var contents []*genai.Content
	var label string
	switch mimeType {
	case "application/pdf":
		raw, err := promptFS.ReadFile(promptContextExtract)
		if err != nil {
			return domainmc.Extraction{}, fmt.Errorf("llm.ExtractContext read: %w", err)
		}
		label = promptContextExtract
		contents = []*genai.Content{genai.NewContentFromParts([]*genai.Part{
			genai.NewPartFromText(string(raw)),
			genai.NewPartFromBytes(data, mimeType),
		}, genai.RoleUser)}
	case "text/plain":
		prompt, err := renderTemplate(promptContextKeyterms, map[string]string{"TEXT": string(data)})
		if err != nil {
			return domainmc.Extraction{}, err
		}
		label = promptContextKeyterms
		contents = genai.Text(prompt)
	default:
		return domainmc.Extraction{}, fmt.Errorf("llm.ExtractContext: unsupported mime %q", mimeType)
	}
	resp, err := c.generateContent(ctx, label, contents, true)
	if err != nil {
		return domainmc.Extraction{}, fmt.Errorf("llm.ExtractContext generate: %w", err)
	}
	var out struct {
		Content  string   `json:"content"`
		Keyterms []string `json:"keyterms"`
	}
	if err := json.Unmarshal([]byte(stripJSONFence(resp.Text())), &out); err != nil {
		return domainmc.Extraction{}, fmt.Errorf("llm.ExtractContext parse: %w", err)
	}
	if len(out.Keyterms) > 50 {
		out.Keyterms = out.Keyterms[:50]
	}
	return domainmc.Extraction{Content: out.Content, Keyterms: out.Keyterms}, nil
}

// completeJSON 同 complete，但要求 application/json 輸出。
func (c *GeminiClient) completeJSON(ctx context.Context, label, prompt string) (string, error) {
	resp, err := c.generateContent(ctx, label, genai.Text(prompt), true)
	if err != nil {
		return "", fmt.Errorf("llm.generate %s: %w", label, err)
	}
	text := resp.Text()
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("llm.generate %s: empty response", label)
	}
	return text, nil
}
