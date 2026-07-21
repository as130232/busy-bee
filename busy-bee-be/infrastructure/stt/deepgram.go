// Deepgram 實作 domain/meeting.STTClient，使用聲學語者分離（diarization）：
// 依聲紋將逐字結果分群為講者（一個聲音＝一位講者），比 LLM 推測式辨識穩定。
// Diarization 僅在同一場錄音內區分講者（A/B/C…），非跨會議身分。
package stt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

const deepgramBaseURL = "https://api.deepgram.com/v1/listen"

type DeepgramClient struct {
	httpClient *http.Client
	apiKey     string
	model      string
	language   string
	keywords   []string
	baseURL    string
}

var _ domainmeeting.STTClient = (*DeepgramClient)(nil)

type DeepgramOption func(*DeepgramClient)

func WithDeepgramBaseURL(u string) DeepgramOption { return func(c *DeepgramClient) { c.baseURL = u } }
func WithDeepgramHTTPClient(hc *http.Client) DeepgramOption {
	return func(c *DeepgramClient) { c.httpClient = hc }
}

func NewDeepgram(apiKey, model, language string, keywords []string, opts ...DeepgramOption) *DeepgramClient {
	c := &DeepgramClient{
		httpClient: &http.Client{Timeout: 10 * time.Minute},
		apiKey:     apiKey,
		model:      model,
		language:   language,
		keywords:   keywords,
		baseURL:    deepgramBaseURL,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type deepgramResponse struct {
	Metadata struct {
		Duration float64 `json:"duration"`
	} `json:"metadata"`
	Results struct {
		Channels []struct {
			Alternatives []struct {
				Words []deepgramWord `json:"words"`
			} `json:"alternatives"`
		} `json:"channels"`
	} `json:"results"`
}

type deepgramWord struct {
	Word           string  `json:"word"`
	PunctuatedWord string  `json:"punctuated_word"`
	Start          float64 `json:"start"`
	End            float64 `json:"end"`
	Speaker        *int    `json:"speaker"`
}

func (c *DeepgramClient) Transcribe(ctx context.Context, audio io.Reader, sizeBytes int64, filename string) (domainmeeting.TranscribeResult, error) {
	q := url.Values{}
	q.Set("model", c.model)
	q.Set("diarize", "true")
	q.Set("punctuate", "true")
	q.Set("smart_format", "true")
	if c.language != "" {
		q.Set("language", c.language)
	}
	for _, kw := range c.keywords {
		q.Add("keywords", kw) // 術語加權，提升專有名詞/英文詞辨識
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"?"+q.Encode(), audio)
	if err != nil {
		return domainmeeting.TranscribeResult{}, fmt.Errorf("stt deepgram request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+c.apiKey)
	req.Header.Set("Content-Type", mimeByExt(filename))
	if sizeBytes > 0 {
		req.ContentLength = sizeBytes // 提供長度避免 chunked，Deepgram 較穩
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domainmeeting.TranscribeResult{}, fmt.Errorf("stt deepgram call: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return domainmeeting.TranscribeResult{}, fmt.Errorf("stt deepgram read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return domainmeeting.TranscribeResult{}, fmt.Errorf("stt deepgram status %d: %.300s", resp.StatusCode, raw)
	}

	var dr deepgramResponse
	if err := json.Unmarshal(raw, &dr); err != nil {
		return domainmeeting.TranscribeResult{}, fmt.Errorf("stt deepgram parse: %w", err)
	}

	segs := aggregateDeepgramWords(dr)
	return domainmeeting.TranscribeResult{
		Text:            domainmeeting.FlattenSegments(segs),
		Segments:        segs,
		DurationSeconds: int(dr.Metadata.Duration + 0.5),
	}, nil
}

// aggregateDeepgramWords 把逐字 words（各帶 speaker 整數）聚合成連續同一講者的段落；
// speaker 0→A、1→B…。中文詞間不加空白，英文詞間加空白。
func aggregateDeepgramWords(dr deepgramResponse) []domainmeeting.TranscriptSegment {
	if len(dr.Results.Channels) == 0 || len(dr.Results.Channels[0].Alternatives) == 0 {
		return nil
	}
	words := dr.Results.Channels[0].Alternatives[0].Words

	var segs []domainmeeting.TranscriptSegment
	var cur *domainmeeting.TranscriptSegment
	for _, w := range words {
		spk := "A"
		if w.Speaker != nil {
			spk = speakerCode(*w.Speaker)
		}
		token := w.PunctuatedWord
		if token == "" {
			token = w.Word
		}
		if cur == nil || cur.Speaker != spk {
			if cur != nil {
				segs = append(segs, *cur)
			}
			cur = &domainmeeting.TranscriptSegment{
				Speaker: spk,
				Text:    token,
				StartMs: int(w.Start * 1000),
				EndMs:   int(w.End * 1000),
			}
			continue
		}
		cur.EndMs = int(w.End * 1000)
		if hasASCIILetter(token) || hasASCIILetter(lastRune(cur.Text)) {
			cur.Text += " " + token
		} else {
			cur.Text += token
		}
	}
	if cur != nil {
		segs = append(segs, *cur)
	}
	return segs
}

func speakerCode(i int) string {
	if i < 0 {
		i = 0
	}
	return string(rune('A' + i%26))
}

func hasASCIILetter(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

func lastRune(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return ""
	}
	return string(r[len(r)-1])
}
