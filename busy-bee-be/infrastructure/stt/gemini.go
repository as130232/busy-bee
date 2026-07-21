// Gemini 實作 domain/meeting.STTClient，並支援 speaker diarization：
// 透過 Files API 上傳音訊，要求模型逐字轉錄並區分講者，回傳分講者片段。
// Diarization 僅在同一場錄音內區分講者（A/B/C…），非跨會議身分。
// Prompt 在 prompts/diarize.md（embedded），調整不需動 client 邏輯。
package stt

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/genai"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

//go:embed prompts/diarize.md
var diarizePrompt string

// filePollInterval 音檔上傳後等待 Gemini 處理（PROCESSING→ACTIVE）的輪詢間隔。
const filePollInterval = 2 * time.Second

type GeminiClient struct {
	client *genai.Client
	model  string
}

var _ domainmeeting.STTClient = (*GeminiClient)(nil)

func NewGemini(ctx context.Context, apiKey, model string) (*GeminiClient, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("stt.NewGemini: %w", err)
	}
	return &GeminiClient{client: client, model: model}, nil
}

// geminiSegment 對應 prompt 要求的 JSON 輸出（start/end 為秒）。
type geminiSegment struct {
	Speaker string  `json:"speaker"`
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
	Text    string  `json:"text"`
}

func (c *GeminiClient) Transcribe(ctx context.Context, audio io.Reader, _ int64, filename string) (domainmeeting.TranscribeResult, error) {
	file, err := c.client.Files.Upload(ctx, audio, &genai.UploadFileConfig{MIMEType: mimeByExt(filename)})
	if err != nil {
		return domainmeeting.TranscribeResult{}, fmt.Errorf("stt gemini upload: %w", err)
	}
	defer func() { _, _ = c.client.Files.Delete(ctx, file.Name, nil) }()

	// 音檔上傳後可能仍在 PROCESSING，輪詢到 ACTIVE 才能用於生成。
	for file.State == genai.FileStateProcessing {
		select {
		case <-ctx.Done():
			return domainmeeting.TranscribeResult{}, ctx.Err()
		case <-time.After(filePollInterval):
		}
		if file, err = c.client.Files.Get(ctx, file.Name, nil); err != nil {
			return domainmeeting.TranscribeResult{}, fmt.Errorf("stt gemini poll file: %w", err)
		}
	}
	if file.State == genai.FileStateFailed {
		return domainmeeting.TranscribeResult{}, fmt.Errorf("stt gemini: file processing failed")
	}

	contents := []*genai.Content{
		genai.NewContentFromParts([]*genai.Part{
			genai.NewPartFromText(diarizePrompt),
			genai.NewPartFromURI(file.URI, file.MIMEType),
		}, genai.RoleUser),
	}
	resp, err := c.client.Models.GenerateContent(ctx, c.model, contents, &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
	})
	if err != nil {
		return domainmeeting.TranscribeResult{}, fmt.Errorf("stt gemini generate: %w", err)
	}

	segs, err := parseGeminiSegments(resp.Text())
	if err != nil {
		return domainmeeting.TranscribeResult{}, err
	}
	return domainmeeting.TranscribeResult{
		Text:            domainmeeting.FlattenSegments(segs),
		Segments:        segs,
		DurationSeconds: durationFromSegments(segs),
	}, nil
}

// parseGeminiSegments 解析模型輸出的 JSON 陣列，容忍被 ```json fence 包裹的情形，
// 並將秒轉為毫秒、去除空白/空段。
func parseGeminiSegments(text string) ([]domainmeeting.TranscriptSegment, error) {
	s := strings.TrimSpace(text)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	var raw []geminiSegment
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil, fmt.Errorf("stt gemini parse segments: %w", err)
	}

	segs := make([]domainmeeting.TranscriptSegment, 0, len(raw))
	for _, r := range raw {
		txt := strings.TrimSpace(r.Text)
		if txt == "" {
			continue
		}
		segs = append(segs, domainmeeting.TranscriptSegment{
			Speaker: strings.TrimSpace(r.Speaker),
			Text:    txt,
			StartMs: int(r.Start * 1000),
			EndMs:   int(r.End * 1000),
		})
	}
	return segs, nil
}

// durationFromSegments 取所有片段最大結束時間（毫秒）換算為秒。
func durationFromSegments(segs []domainmeeting.TranscriptSegment) int {
	maxEnd := 0
	for _, s := range segs {
		if s.EndMs > maxEnd {
			maxEnd = s.EndMs
		}
	}
	return maxEnd / 1000
}

func mimeByExt(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".mp3":
		return "audio/mpeg"
	case ".m4a", ".mp4":
		return "audio/mp4"
	case ".wav":
		return "audio/wav"
	case ".ogg", ".opus":
		return "audio/ogg"
	case ".webm":
		return "audio/webm"
	case ".flac":
		return "audio/flac"
	case ".aac":
		return "audio/aac"
	default:
		return "audio/mpeg"
	}
}
