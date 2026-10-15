package llm

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/genai"

	"github.com/as130232/busy-bee/busy-bee-be/domain/correction"
)

//go:embed prompts/correct_transcript.md
var correctPrompt string

// correctFilePollInterval 音訊片段上傳後等待 Gemini 處理（PROCESSING→ACTIVE）的輪詢間隔。
const correctFilePollInterval = 2 * time.Second

var _ correction.Corrector = (*GeminiClient)(nil)

// CorrectChunk 上傳音訊片段（Files API）＋逐字稿段落，回傳模型提出的修正
// （未驗證，由 application 層 ValidatePatches 過濾）。
func (c *GeminiClient) CorrectChunk(ctx context.Context, in correction.CorrectorInput) ([]correction.Patch, error) {
	file, err := c.client.Files.Upload(ctx, in.Audio, &genai.UploadFileConfig{MIMEType: "audio/mpeg"})
	if err != nil {
		return nil, fmt.Errorf("llm.CorrectChunk upload: %w", err)
	}
	defer func() { _, _ = c.client.Files.Delete(context.WithoutCancel(ctx), file.Name, nil) }()

	for file.State == genai.FileStateProcessing {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(correctFilePollInterval):
		}
		if file, err = c.client.Files.Get(ctx, file.Name, nil); err != nil {
			return nil, fmt.Errorf("llm.CorrectChunk poll: %w", err)
		}
	}
	if file.State == genai.FileStateFailed {
		return nil, fmt.Errorf("llm.CorrectChunk: file processing failed")
	}

	contents := []*genai.Content{genai.NewContentFromParts([]*genai.Part{
		genai.NewPartFromText(buildCorrectionPrompt(in)),
		genai.NewPartFromURI(file.URI, file.MIMEType),
	}, genai.RoleUser)}
	resp, err := c.generateContent(ctx, "correct_transcript", contents, true)
	if err != nil {
		return nil, fmt.Errorf("llm.CorrectChunk generate: %w", err)
	}
	return parsePatches(resp.Text())
}

// buildCorrectionPrompt 上文以「講者: 文字」列出；待校正段以「[i] (🔒 )(mm:ss) 講者: 文字」列出（mm:ss 為片段內相對時間），
// 文字用 AnalysisText（含低信心標記），讓模型知道哪裡最需要細聽。
func buildCorrectionPrompt(in correction.CorrectorInput) string {
	var ctxB, segB strings.Builder
	for _, s := range in.Context {
		fmt.Fprintf(&ctxB, "%s: %s\n", s.Speaker, s.Text)
	}
	for _, s := range in.Segments {
		lock := ""
		if s.Locked {
			lock = "🔒 "
		}
		sec := max(0, s.Segment.StartMs-in.ClipStartMs) / 1000 // 片段內相對時間，與音訊對齊
		fmt.Fprintf(&segB, "[%d] %s(%02d:%02d) %s: %s\n", s.I, lock, sec/60, sec%60, s.Segment.Speaker, s.Segment.AnalysisText())
	}
	p := strings.ReplaceAll(correctPrompt, "{{CONTEXT}}", orNone(ctxB.String()))
	return withBackground(in.Background, strings.ReplaceAll(p, "{{SEGMENTS}}", strings.TrimSpace(segB.String())))
}

// parsePatches 解析模型輸出的 JSON 陣列（容忍 ```json fence）；無法解析時包 correction.ErrBadOutput。
func parsePatches(text string) ([]correction.Patch, error) {
	var out []correction.Patch
	if err := json.Unmarshal([]byte(stripJSONFence(text)), &out); err != nil {
		return nil, fmt.Errorf("llm.parsePatches: %w: %v", correction.ErrBadOutput, err)
	}
	return out, nil
}
