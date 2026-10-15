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
	"strconv"
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
	// 術語加權，提升專有名詞/英文詞辨識。nova-3 不支援 keywords，改用 keyterm（不帶權重）。
	useKeyterm := strings.HasPrefix(model, "nova-3")
	for _, kw := range keywords {
		if useKeyterm {
			q.Add("keyterm", stripKeywordWeight(kw))
		} else {
			q.Add("keywords", kw)
		}
	}
	return q
}

// stripKeywordWeight 去除 keywords 格式的「:權重」後綴（如 "Kubernetes:2" → "Kubernetes"）；
// 冒號後不是數字時視為術語本身的一部分，原樣保留。
func stripKeywordWeight(kw string) string {
	i := strings.LastIndex(kw, ":")
	if i < 0 {
		return kw
	}
	if _, err := strconv.ParseFloat(kw[i+1:], 64); err != nil {
		return kw
	}
	return kw[:i]
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
	// Confidence 0~1，辨識信心；以指標區分「未回傳」與「真的是 0」。
	Confidence *float64 `json:"confidence"`
}

func (c *DeepgramClient) Transcribe(ctx context.Context, audio io.Reader, sizeBytes int64, filename string, language domainmeeting.Language, keyterms []string) (domainmeeting.TranscribeResult, error) {
	lang := string(language)
	if lang == "" {
		lang = c.language // 呼叫端未帶語言時，退回 config 注入的 fallback 預設值
	}
	q := buildDeepgramQuery(c.model, lang, mergeKeywords(c.keywords, keyterms))

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

	autoZh := lang == string(domainmeeting.LanguageAuto) && strings.HasPrefix(detectedLanguage(dr), "zh")
	segs := aggregateDeepgramWords(dr, strings.HasPrefix(lang, "zh") || autoZh)
	// 只有 auto 模式且偵測結果為中文時才做簡轉繁：明確指定 zh-TW 時 Deepgram 已直接輸出繁體，
	// 再轉一次徒增成本也可能誤判，故僅在 auto 分支後製。
	if autoZh {
		for i := range segs {
			segs[i].Text = toTraditional(segs[i].Text)
			if segs[i].MarkedText != "" {
				segs[i].MarkedText = toTraditional(segs[i].MarkedText)
			}
		}
	}
	return domainmeeting.TranscribeResult{
		Text:            domainmeeting.FlattenSegments(segs),
		Segments:        segs,
		DurationSeconds: int(dr.Metadata.Duration + 0.5),
	}, nil
}

// lowConfidenceThreshold 字詞 confidence 低於此值視為辨識不可靠，在 MarkedText 中以「〔?…〕」標記。
const lowConfidenceThreshold = 0.6

// fragmentGapSec 中文模式下，英文字詞與前一個英文字詞的間隔小於此值（秒）視為同一個單字被切開的片段。
const fragmentGapSec = 0.01

// segmentBuilder 累積同一講者連續字詞，同步組出純文字與帶低信心標記的文字。
type segmentBuilder struct {
	seg    domainmeeting.TranscriptSegment
	marked strings.Builder
	inLow  bool // 目前是否在一段未關閉的低信心標記中
	hasLow bool
	// joinFragments 中文模式：Deepgram 會把中文裡夾的英文單字切成時間相連的片段
	// （如 Kubernetes → "k","er","net","es"），此時相連片段直接接合、不加空白。
	joinFragments bool
	lastEnd       float64
}

func (b *segmentBuilder) add(token string, low bool, start, end float64) {
	sep := ""
	if b.seg.Text != "" && (hasASCIILetter(token) || hasASCIILetter(lastRune(b.seg.Text))) {
		sep = " " // 中文詞間不加空白，英文詞間加空白
		if b.joinFragments && hasASCIILetter(token) && hasASCIILetter(lastRune(b.seg.Text)) && start-b.lastEnd < fragmentGapSec {
			sep = ""
		}
	}
	b.lastEnd = end
	b.seg.Text += sep + token
	switch {
	case low && !b.inLow:
		b.marked.WriteString(sep + "〔?" + token)
		b.inLow, b.hasLow = true, true
	case !low && b.inLow:
		b.marked.WriteString("〕" + sep + token)
		b.inLow = false
	default:
		b.marked.WriteString(sep + token)
	}
}

func (b *segmentBuilder) finish() domainmeeting.TranscriptSegment {
	if b.hasLow {
		if b.inLow {
			b.marked.WriteString("〕")
		}
		b.seg.MarkedText = b.marked.String()
	}
	return b.seg
}

// aggregateDeepgramWords 把逐字 words（各帶 speaker 整數）聚合成連續同一講者的段落；
// speaker 0→A、1→B…。中文詞間不加空白，英文詞間加空白。
// 低信心字詞（confidence < lowConfidenceThreshold）另記於 MarkedText，供 LLM 判斷哪裡可能聽錯。
// zh 為 true（中文音訊）時，時間相連的英文片段接合成一個單字。
func aggregateDeepgramWords(dr deepgramResponse, zh bool) []domainmeeting.TranscriptSegment {
	if len(dr.Results.Channels) == 0 || len(dr.Results.Channels[0].Alternatives) == 0 {
		return nil
	}
	words := dr.Results.Channels[0].Alternatives[0].Words

	var segs []domainmeeting.TranscriptSegment
	var cur *segmentBuilder
	for _, w := range words {
		spk := "A"
		if w.Speaker != nil {
			spk = speakerCode(*w.Speaker)
		}
		token := w.PunctuatedWord
		if token == "" {
			token = w.Word
		}
		if cur == nil || cur.seg.Speaker != spk {
			if cur != nil {
				segs = append(segs, cur.finish())
			}
			cur = &segmentBuilder{seg: domainmeeting.TranscriptSegment{Speaker: spk, StartMs: int(w.Start * 1000)}, joinFragments: zh}
		}
		cur.seg.EndMs = int(w.End * 1000)
		// 無 confidence 欄位時視為可信，避免整段被誤標
		cur.add(token, w.Confidence != nil && *w.Confidence < lowConfidenceThreshold, w.Start, w.End)
	}
	if cur != nil {
		segs = append(segs, cur.finish())
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

// mergeKeywords 全域術語（可帶權重）＋該場會前資料術語：不分大小寫去重、全域優先、上限 100。
func mergeKeywords(global, extra []string) []string {
	const limit = 100
	seen := map[string]bool{}
	out := make([]string, 0, len(global)+len(extra))
	for _, list := range [][]string{global, extra} {
		for _, kw := range list {
			kw = strings.TrimSpace(kw)
			k := strings.ToLower(stripKeywordWeight(kw))
			if kw == "" || seen[k] || len(out) >= limit {
				continue
			}
			seen[k] = true
			out = append(out, kw)
		}
	}
	return out
}
