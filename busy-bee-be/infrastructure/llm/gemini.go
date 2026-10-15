// Package llm 以 Gemini 實作 domain/meeting.Summarizer 與 domain/actionitem.Extractor。
// Prompt 模板在 prompts/（embedded），調整模板不需動 client 邏輯。
package llm

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
	// fallback 主模型回 404/NOT_FOUND（如 Preview 模型下架）時改用的模型；空字串代表不備援。
	fallback string
	// gen 實際呼叫 Gemini 的函式（預設 client.Models.GenerateContent；測試可替換）。
	gen func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error)
}

// Option GeminiClient 選項。
type Option func(*GeminiClient)

// WithFallbackModel 主模型不存在（404/NOT_FOUND，例如 Preview 下架）時自動改用的模型。
func WithFallbackModel(model string) Option { return func(c *GeminiClient) { c.fallback = model } }

var (
	_ domainactionitem.Extractor = (*GeminiClient)(nil)
	_ domainmeeting.Summarizer   = (*GeminiClient)(nil)
	_ domainsearch.Answerer      = (*GeminiClient)(nil)
)

func NewGemini(ctx context.Context, apiKey, model string, opts ...Option) (*GeminiClient, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("llm.NewGemini: %w", err)
	}
	c := &GeminiClient{client: client, model: model}
	c.gen = client.Models.GenerateContent
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// Extract 於同一次呼叫產出一句話摘要與行動項。prompt 要求輸出 JSON 物件
// （即使無行動項也回 {"summary":..., "actionItems":[]}），故不會被空回應檢查誤判。
// now（會議日期）注入 {{TODAY}}，供模型把「下週五」等相對時限推算成絕對日期 dueISO。
func (c *GeminiClient) Extract(ctx context.Context, transcript string, now time.Time, background string) (domainactionitem.Extraction, error) {
	return c.ExtractExcluding(ctx, transcript, now, nil, background)
}

// ExtractExcluding 同 Extract，但告知模型 existing 中的行動項已存在、不得重複輸出（AI 校正重抽用）。
func (c *GeminiClient) ExtractExcluding(ctx context.Context, transcript string, now time.Time, existing []string, background string) (domainactionitem.Extraction, error) {
	prompt, err := buildPrompt(promptActionItems, transcript)
	if err != nil {
		return domainactionitem.Extraction{}, err
	}
	prompt = withBackground(background, prompt)
	prompt = strings.ReplaceAll(prompt, "{{TODAY}}", now.Format("2006-01-02"))
	prompt = strings.ReplaceAll(prompt, "{{EXISTING}}", existingBlock(existing))
	text, err := c.complete(ctx, promptActionItems, prompt)
	if err != nil {
		return domainactionitem.Extraction{}, err
	}
	return parseExtraction(text)
}

// existingBlock 既有行動項清單（每行「- 描述」）；無則回「（無）」。
func existingBlock(existing []string) string {
	if len(existing) == 0 {
		return "（無）"
	}
	var b strings.Builder
	for _, e := range existing {
		fmt.Fprintf(&b, "- %s\n", e)
	}
	return strings.TrimSpace(b.String())
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
func (c *GeminiClient) Summarize(ctx context.Context, transcript string, scenario domainmeeting.Scenario, background string) (domainmeeting.SummaryResult, error) {
	tmpl, ok := scenarioPrompts[scenario]
	if !ok {
		tmpl = scenarioPrompts[domainmeeting.ScenarioMeeting]
	}
	prompt, err := buildPrompt(tmpl, transcript)
	if err != nil {
		return domainmeeting.SummaryResult{}, err
	}
	text, err := c.complete(ctx, tmpl, withBackground(background, prompt))
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

// backgroundRule 會後流程使用會前資料時附加的規則：背景只用來理解，內容只能來自逐字稿。
const backgroundRule = "（以上背景資料只用來理解專有名詞、術語與脈絡、修正同音錯字；整理出的內容只能來自逐字稿，不得把背景資料寫成會議中說過的內容。）"

// withBackground 把會前資料放在 prompt 最前面（前綴相同利於隱性快取）並附規則；無背景資料時原樣回傳。
func withBackground(background, prompt string) string {
	if strings.TrimSpace(background) == "" {
		return prompt
	}
	return strings.TrimSpace(background) + "\n\n" + backgroundRule + "\n\n---\n\n" + prompt
}

// orNone 空白內容以「（無）」代入，讓 prompt 讀起來完整。
func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "（無）"
	}
	return strings.TrimSpace(s)
}

// generateConfig 依模型決定生成設定。gemini-3 系列預設 thinking=high，思考 token 以輸出價計費，
// 對摘要/抽取這類任務成本高而效益有限，故一律降為 low；其他模型（含 gemini-flash-latest 等別名）
// 不確定是否支援 thinkingLevel，維持供應商預設。
func generateConfig(model string) *genai.GenerateContentConfig {
	if !strings.HasPrefix(model, "gemini-3") {
		return nil
	}
	return &genai.GenerateContentConfig{
		ThinkingConfig: &genai.ThinkingConfig{ThinkingLevel: genai.ThinkingLevelLow},
	}
}

// generateContent 以主模型生成；主模型不存在（404/NOT_FOUND，如 Preview 下架）且有設定備援時改用備援模型。
// 其他錯誤（含 429/5xx）原樣回傳，交由佇列 retry。jsonOut 時要求 application/json 輸出。
func (c *GeminiClient) generateContent(ctx context.Context, label string, contents []*genai.Content, jsonOut bool) (*genai.GenerateContentResponse, error) {
	resp, err := c.gen(ctx, c.model, contents, buildConfig(c.model, jsonOut))
	if err == nil || c.fallback == "" || c.fallback == c.model || !isModelNotFound(err) {
		return resp, err
	}
	slog.WarnContext(ctx, "llm.model_fallback", "label", label, "model", c.model, "fallback", c.fallback, "err", err)
	return c.gen(ctx, c.fallback, contents, buildConfig(c.fallback, jsonOut))
}

// buildConfig 依模型組生成設定（thinking 等級見 generateConfig），jsonOut 時加上 JSON 輸出。
func buildConfig(model string, jsonOut bool) *genai.GenerateContentConfig {
	cfg := generateConfig(model)
	if jsonOut {
		if cfg == nil {
			cfg = &genai.GenerateContentConfig{}
		}
		cfg.ResponseMIMEType = "application/json"
	}
	return cfg
}

// isModelNotFound 判斷是否為「模型不存在／不支援」（Gemini API 回 404 NOT_FOUND）。
func isModelNotFound(err error) bool {
	var apiErr genai.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == 404 || apiErr.Status == "NOT_FOUND"
	}
	return false
}

// complete 送出已組好的 prompt 並回傳非空文字回應（空回應視為錯誤）。
func (c *GeminiClient) complete(ctx context.Context, label, prompt string) (string, error) {
	resp, err := c.generateContent(ctx, label, genai.Text(prompt), false)
	if err != nil {
		return "", fmt.Errorf("llm.generate %s: %w", label, err)
	}
	text := resp.Text()
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("llm.generate %s: empty response", label)
	}
	return text, nil
}
