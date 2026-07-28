// Package llm 以 Gemini 實作 domain/meeting.Summarizer 與 domain/actionitem.Extractor。
// Prompt 模板在 prompts/（embedded），調整模板不需動 client 邏輯。
package llm

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/genai"

	domainactionitem "github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainsearch "github.com/as130232/busy-bee/busy-bee-be/domain/search"
)

//go:embed prompts/*.md
var promptFS embed.FS

const promptActionItems = "prompts/action_items.md"

const promptCrossMeetingQA = "prompts/cross_meeting_qa.md"

// scenarioPrompts 每個情境對應的結構化摘要 prompt 模板。
var scenarioPrompts = map[domainmeeting.Scenario]string{
	domainmeeting.ScenarioMeeting:   "prompts/summary_meeting.md",
	domainmeeting.ScenarioCasual:    "prompts/summary_casual.md",
	domainmeeting.ScenarioInterview: "prompts/summary_interview.md",
	domainmeeting.ScenarioIdea:      "prompts/summary_idea.md",
}

func buildPrompt(templatePath, transcript string) (string, error) {
	raw, err := promptFS.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("llm.buildPrompt read %s: %w", templatePath, err)
	}
	return strings.ReplaceAll(string(raw), "{{TRANSCRIPT}}", transcript), nil
}

type GeminiClient struct {
	client *genai.Client
	model  string
}

var (
	_ domainactionitem.Extractor = (*GeminiClient)(nil)
	_ domainmeeting.Summarizer   = (*GeminiClient)(nil)
	_ domainsearch.Answerer      = (*GeminiClient)(nil)
)

func NewGemini(ctx context.Context, apiKey, model string) (*GeminiClient, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("llm.NewGemini: %w", err)
	}
	return &GeminiClient{client: client, model: model}, nil
}

// Extract 於同一次呼叫產出一句話摘要與行動項。prompt 要求輸出 JSON 物件
// （即使無行動項也回 {"summary":..., "actionItems":[]}），故不會被空回應檢查誤判。
// now（會議日期）注入 {{TODAY}}，供模型把「下週五」等相對時限推算成絕對日期 dueISO。
func (c *GeminiClient) Extract(ctx context.Context, transcript string, now time.Time) (domainactionitem.Extraction, error) {
	prompt, err := buildPrompt(promptActionItems, transcript)
	if err != nil {
		return domainactionitem.Extraction{}, err
	}
	prompt = strings.ReplaceAll(prompt, "{{TODAY}}", now.Format("2006-01-02"))
	text, err := c.complete(ctx, promptActionItems, prompt)
	if err != nil {
		return domainactionitem.Extraction{}, err
	}
	return parseExtraction(text)
}

// parseExtraction 解析模型輸出的 JSON 物件，容忍被 ```json fence 包裹的情形。
func parseExtraction(text string) (domainactionitem.Extraction, error) {
	s := stripJSONFence(text)
	var out domainactionitem.Extraction
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return domainactionitem.Extraction{}, fmt.Errorf("llm.parseExtraction: %w", err)
	}
	return out, nil
}

// stripJSONFence 去除模型輸出可能包裹的 ```json fence 與前後空白。
func stripJSONFence(text string) string {
	s := strings.TrimSpace(text)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// Summarize 依情境選 prompt，一次呼叫產出結構化摘要區塊。
// 情境無對應模板時回退會議模板（ParseScenario 已保證有效值，此處為雙保險）。
func (c *GeminiClient) Summarize(ctx context.Context, transcript string, scenario domainmeeting.Scenario) (domainmeeting.SummaryResult, error) {
	tmpl, ok := scenarioPrompts[scenario]
	if !ok {
		tmpl = scenarioPrompts[domainmeeting.ScenarioMeeting]
	}
	text, err := c.generate(ctx, tmpl, transcript)
	if err != nil {
		return domainmeeting.SummaryResult{}, err
	}
	var out struct {
		Sections []domainmeeting.SummarySection `json:"sections"`
		Tags     []string                       `json:"tags"`
	}
	if err := json.Unmarshal([]byte(stripJSONFence(text)), &out); err != nil {
		return domainmeeting.SummaryResult{}, fmt.Errorf("llm.Summarize parse: %w", err)
	}
	return domainmeeting.SummaryResult{Sections: out.Sections, Tags: out.Tags}, nil
}

// Answer 依問題、編號內容來源與會議清單生成帶引用的繁中答案。
// sources 以「[n] 標題\n內容」注入 {{SOURCES}}（內容型問題，[n] 標註出處）；
// meetings 以「- 日期｜標題：摘要」注入 {{MEETINGS}}（列舉/時間型問題）；
// {{TODAY}}（UTC+8 今日）供模型判讀「這週/上週」；prompt 要求僅依資料作答、不杜撰。
func (c *GeminiClient) Answer(ctx context.Context, question string, sources []domainsearch.QASource, meetings []domainsearch.MeetingBrief) (string, error) {
	raw, err := promptFS.ReadFile(promptCrossMeetingQA)
	if err != nil {
		return "", fmt.Errorf("llm.Answer read %s: %w", promptCrossMeetingQA, err)
	}

	var srcB strings.Builder
	for _, s := range sources {
		fmt.Fprintf(&srcB, "[%d] %s\n%s\n\n", s.Index, s.Title, s.Content)
	}
	var metB strings.Builder
	for _, m := range meetings {
		fmt.Fprintf(&metB, "- %s｜%s", m.Date, m.Title)
		if strings.TrimSpace(m.Summary) != "" {
			fmt.Fprintf(&metB, "：%s", m.Summary)
		}
		metB.WriteString("\n")
	}

	today := time.Now().UTC().Add(8 * time.Hour).Format("2006-01-02")
	prompt := strings.ReplaceAll(string(raw), "{{SOURCES}}", orNone(srcB.String()))
	prompt = strings.ReplaceAll(prompt, "{{MEETINGS}}", orNone(metB.String()))
	prompt = strings.ReplaceAll(prompt, "{{QUESTION}}", question)
	prompt = strings.ReplaceAll(prompt, "{{TODAY}}", today)
	return c.complete(ctx, promptCrossMeetingQA, prompt)
}

// orNone 空白內容以「（無）」代入，讓 prompt 讀起來完整。
func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "（無）"
	}
	return strings.TrimSpace(s)
}

func (c *GeminiClient) generate(ctx context.Context, templatePath, transcript string) (string, error) {
	prompt, err := buildPrompt(templatePath, transcript)
	if err != nil {
		return "", err
	}
	return c.complete(ctx, templatePath, prompt)
}

// complete 送出已組好的 prompt 並回傳非空文字回應（空回應視為錯誤）。
func (c *GeminiClient) complete(ctx context.Context, label, prompt string) (string, error) {
	resp, err := c.client.Models.GenerateContent(ctx, c.model, genai.Text(prompt), nil)
	if err != nil {
		return "", fmt.Errorf("llm.generate %s: %w", label, err)
	}
	text := resp.Text()
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("llm.generate %s: empty response", label)
	}
	return text, nil
}
