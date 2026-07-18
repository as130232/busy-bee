// Package llm 以 Gemini 實作 domain/artifact.LLMClient。
// Prompt 模板在 prompts/（embedded），調整模板不需動 client 邏輯。
package llm

import (
	"context"
	"embed"
	"fmt"
	"strings"

	"google.golang.org/genai"

	domainartifact "github.com/as130232/busy-bee/busy-bee-be/domain/artifact"
)

//go:embed prompts/*.md
var promptFS embed.FS

const (
	promptPRD      = "prompts/prd.md"
	promptTechSpec = "prompts/tech_spec.md"
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

var _ domainartifact.LLMClient = (*GeminiClient)(nil)

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
