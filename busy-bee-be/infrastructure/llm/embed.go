package llm

import (
	"context"
	"fmt"

	"google.golang.org/genai"

	domainsearch "github.com/as130232/busy-bee/busy-bee-be/domain/search"
)

const (
	embedModel = "gemini-embedding-001"
	embedDim   = 768
)

// GeminiClient 實作 search.Embedder（維度鎖定 768；換維度需重 embed 全部）。
var _ domainsearch.Embedder = (*GeminiClient)(nil)

// Embed 單段文字轉 768 維向量。
func (c *GeminiClient) Embed(ctx context.Context, text string) ([]float32, error) {
	dim := int32(embedDim)
	resp, err := c.client.Models.EmbedContent(ctx, embedModel,
		genai.Text(text),
		&genai.EmbedContentConfig{OutputDimensionality: &dim},
	)
	if err != nil {
		return nil, fmt.Errorf("llm.Embed: %w", err)
	}
	if len(resp.Embeddings) == 0 || len(resp.Embeddings[0].Values) == 0 {
		return nil, fmt.Errorf("llm.Embed: empty embedding")
	}
	return resp.Embeddings[0].Values, nil
}
