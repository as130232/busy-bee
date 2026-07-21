// Package llm 以 Gemini 實作 domain/artifact.LLMClient。
// Prompt 模板在 prompts/（embedded），調整模板不需動 client 邏輯。
package llm

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"

	domainactionitem "github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	domainartifact "github.com/as130232/busy-bee/busy-bee-be/domain/artifact"
)

//go:embed prompts/*.md
var promptFS embed.FS

const (
	promptPRD         = "prompts/prd.md"
	promptTechSpec    = "prompts/tech_spec.md"
	promptActionItems = "prompts/action_items.md"
)

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
	_ domainartifact.LLMClient   = (*GeminiClient)(nil)
	_ domainactionitem.Extractor = (*GeminiClient)(nil)
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

func (c *GeminiClient) GeneratePRD(ctx context.Context, transcript string) (string, error) {
	return c.generate(ctx, promptPRD, transcript)
}

func (c *GeminiClient) GenerateTechSpec(ctx context.Context, transcript string) (string, error) {
	return c.generate(ctx, promptTechSpec, transcript)
}

// Extract 於同一次呼叫產出一句話摘要與行動項。prompt 要求輸出 JSON 物件
// （即使無行動項也回 {"summary":..., "actionItems":[]}），故不會被空回應檢查誤判。
func (c *GeminiClient) Extract(ctx context.Context, transcript string) (domainactionitem.Extraction, error) {
	text, err := c.generate(ctx, promptActionItems, transcript)
	if err != nil {
		return domainactionitem.Extraction{}, err
	}
	return parseExtraction(text)
}

// parseExtraction 解析模型輸出的 JSON 物件，容忍被 ```json fence 包裹的情形。
func parseExtraction(text string) (domainactionitem.Extraction, error) {
	s := strings.TrimSpace(text)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	var out domainactionitem.Extraction
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return domainactionitem.Extraction{}, fmt.Errorf("llm.parseExtraction: %w", err)
	}
	return out, nil
}

func (c *GeminiClient) generate(ctx context.Context, templatePath, transcript string) (string, error) {
	prompt, err := buildPrompt(templatePath, transcript)
	if err != nil {
		return "", err
	}

	resp, err := c.client.Models.GenerateContent(ctx, c.model, genai.Text(prompt), nil)
	if err != nil {
		return "", fmt.Errorf("llm.generate %s: %w", templatePath, err)
	}
	text := resp.Text()
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("llm.generate %s: empty response", templatePath)
	}
	return text, nil
}
