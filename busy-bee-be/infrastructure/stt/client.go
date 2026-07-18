// Package stt 以 Groq Whisper 實作 domain/meeting.STTClient。
// 超過上傳上限的音訊自動以 ffmpeg 壓縮為低 bitrate mono mp3（語音辨識對音質不敏感）。
package stt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

const (
	defaultBaseURL        = "https://api.groq.com/openai/v1"
	defaultModel          = "whisper-large-v3"
	defaultMaxUploadBytes = 25 * 1024 * 1024 // Groq free tier 上限

	// transcribePrompt 引導 Whisper 中文輸出使用繁體（不影響語言自動偵測）。
	transcribePrompt = "以下是繁體中文的會議逐字稿，可能夾雜英文技術術語。"
)

type Client struct {
	httpClient     *http.Client
	apiKey         string
	baseURL        string
	model          string
	maxUploadBytes int64
}

var _ domainmeeting.STTClient = (*Client)(nil)

type Option func(*Client)

func WithBaseURL(url string) Option          { return func(c *Client) { c.baseURL = url } }
func WithMaxUploadBytes(n int64) Option      { return func(c *Client) { c.maxUploadBytes = n } }
func WithModel(model string) Option          { return func(c *Client) { c.model = model } }
func WithHTTPClient(hc *http.Client) Option  { return func(c *Client) { c.httpClient = hc } }

func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		httpClient:     &http.Client{Timeout: 10 * time.Minute},
		apiKey:         apiKey,
		baseURL:        defaultBaseURL,
		model:          defaultModel,
		maxUploadBytes: defaultMaxUploadBytes,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type transcriptionResponse struct {
	Text     string  `json:"text"`
	Duration float64 `json:"duration"`
	Segments []struct {
		Text string `json:"text"`
	} `json:"segments"`
}

// assembleText 以分段組稿並去除 Whisper 的相鄰重複幻覺：
// 段落與前一段相同、或為前一段的尾部子串時捨棄。無分段資料時退回整段 text。
func assembleText(tr transcriptionResponse) string {
	if len(tr.Segments) == 0 {
		return strings.TrimSpace(tr.Text)
	}
	var parts []string
	prev := ""
	for _, seg := range tr.Segments {
		s := strings.TrimSpace(seg.Text)
		if s == "" {
			continue
		}
		if s == prev || (prev != "" && strings.HasSuffix(prev, s)) {
			continue
		}
		parts = append(parts, s)
		prev = s
	}
	return strings.Join(parts, " ")
}

type errorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) Transcribe(ctx context.Context, audio io.Reader, sizeBytes int64, filename string) (domainmeeting.TranscribeResult, error) {
	if sizeBytes > c.maxUploadBytes {
		compressed, compressedSize, err := CompressToMP3(ctx, audio)
		if err != nil {
			return domainmeeting.TranscribeResult{}, fmt.Errorf("stt compress: %w", err)
		}
		defer compressed.Close()
		if compressedSize > c.maxUploadBytes {
			return domainmeeting.TranscribeResult{}, fmt.Errorf("stt: audio still exceeds %d bytes after compression", c.maxUploadBytes)
		}
		audio = compressed
		filename = strings.TrimSuffix(filename, ext(filename)) + ".mp3"
	}

	body, contentType, err := buildMultipart(audio, filename, c.model)
	if err != nil {
		return domainmeeting.TranscribeResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/audio/transcriptions", body)
	if err != nil {
		return domainmeeting.TranscribeResult{}, fmt.Errorf("stt request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", contentType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domainmeeting.TranscribeResult{}, fmt.Errorf("stt call: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return domainmeeting.TranscribeResult{}, fmt.Errorf("stt read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var er errorResponse
		if json.Unmarshal(raw, &er) == nil && er.Error.Message != "" {
			return domainmeeting.TranscribeResult{}, fmt.Errorf("stt groq %d: %s", resp.StatusCode, er.Error.Message)
		}
		return domainmeeting.TranscribeResult{}, fmt.Errorf("stt groq status %d", resp.StatusCode)
	}

	var tr transcriptionResponse
	if err := json.Unmarshal(raw, &tr); err != nil {
		return domainmeeting.TranscribeResult{}, fmt.Errorf("stt parse response: %w", err)
	}
	return domainmeeting.TranscribeResult{
		Text:            assembleText(tr),
		DurationSeconds: int(math.Round(tr.Duration)),
	}, nil
}

func buildMultipart(audio io.Reader, filename, model string) (io.Reader, string, error) {
	// 音訊可達數十 MB，用 pipe 串流避免整份載入記憶體
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		err := func() error {
			if err := mw.WriteField("model", model); err != nil {
				return err
			}
			if err := mw.WriteField("response_format", "verbose_json"); err != nil {
				return err
			}
			if err := mw.WriteField("prompt", transcribePrompt); err != nil {
				return err
			}
			fw, err := mw.CreateFormFile("file", filename)
			if err != nil {
				return err
			}
			if _, err := io.Copy(fw, audio); err != nil {
				return err
			}
			return mw.Close()
		}()
		pw.CloseWithError(err)
	}()
	return pr, mw.FormDataContentType(), nil
}

func ext(filename string) string {
	if i := strings.LastIndex(filename, "."); i >= 0 {
		return filename[i:]
	}
	return ""
}
