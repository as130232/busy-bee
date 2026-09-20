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
	"strings"
	"time"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

const deepgramBaseURL = "https://api.deepgram.com/v1/listen"

// buildDeepgramQuery 組裝 Deepgram API 的 query 參數。
// lang == "auto" 時用 detect_language=true（不用 language=multi：對純中文音訊會壞掉，已實測確認）；
// lang 為其他非空值時明確帶 language；lang 為空時兩者皆不帶，交由 Deepgram 預設。
func buildDeepgramQuery(model, lang string, keywords []string) url.Values {
	q := url.Values{}
	q.Set("model", model)
	q.Set("diarize", "true")
	q.Set("punctuate", "true")
	q.Set("smart_format", "true")
	switch domainmeeting.Language(lang) {
	case domainmeeting.LanguageAuto:
		q.Set("detect_language", "true")
	case "":
		// 不帶語言參數
	default:
		q.Set("language", lang)
	}
	for _, kw := range keywords {
		q.Add("keywords", kw) // 術語加權，提升專有名詞/英文詞辨識
	}
	return q
}

type DeepgramClient struct {
	httpClient *http.Client
	apiKey     string
	model      string
	// language 是 config 注入的 fallback 預設值；per-meeting 語言由 Transcribe 呼叫時的參數決定，
	// 只有呼叫端未帶語言（空字串）才會用到這個值。
	language string
	keywords []string
	baseURL  string
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
			// DetectedLanguage 僅在請求帶 detect_language=true（language=auto）時有值。
			DetectedLanguage string `json:"detected_language"`
			Alternatives     []struct {
				Words []deepgramWord `json:"words"`
			} `json:"alternatives"`
		} `json:"channels"`
	} `json:"results"`
}

// detectedLanguage 回傳 Deepgram 回應中偵測到的語言（僅 auto 模式有值），無資料時回傳空字串。
func detectedLanguage(dr deepgramResponse) string {
	if len(dr.Results.Channels) == 0 {
		return ""
	}
	return dr.Results.Channels[0].DetectedLanguage
}

type deepgramWord struct {
	Word           string  `json:"word"`
	PunctuatedWord string  `json:"punctuated_word"`
	Start          float64 `json:"start"`
	End            float64 `json:"end"`
	Speaker        *int    `json:"speaker"`
}

func (c *DeepgramClient) Transcribe(ctx context.Context, audio io.Reader, sizeBytes int64, filename string, language domainmeeting.Language) (domainmeeting.TranscribeResult, error) {
	lang := string(language)
	if lang == "" {
		lang = c.language // 呼叫端未帶語言時，退回 config 注入的 fallback 預設值
	}
	q := buildDeepgramQuery(c.model, lang, c.keywords)

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
	// 只有 auto 模式且偵測結果為中文時才做簡轉繁：明確指定 zh-TW 時 Deepgram 已直接輸出繁體，
	// 再轉一次徒增成本也可能誤判，故僅在 auto 分支後製。
	if lang == string(domainmeeting.LanguageAuto) && strings.HasPrefix(detectedLanguage(dr), "zh") {
		for i := range segs {
			segs[i].Text = toTraditional(segs[i].Text)
		}
	}
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
