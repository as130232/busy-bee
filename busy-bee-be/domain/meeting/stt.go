package meeting

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// TranscriptSegment 一段由單一講者連續說出的逐字稿。
// Speaker 為同一場錄音內的講者代號（"A" / "B" / "C" …），非跨會議身分。
type TranscriptSegment struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
	StartMs int    `json:"startMs"`
	EndMs   int    `json:"endMs"`
	// MarkedText 與 Text 同內容，但 STT 低信心字詞以「〔?…〕」包住，僅供 LLM 分析時知道哪裡不可靠；
	// 段落無低信心字詞、或使用者手動編輯過該段時為空（此時以 Text 為準）。不對前端輸出。
	MarkedText string `json:"markedText,omitempty"`
	// ManuallyEdited 使用者手動修正過此段；AI 校正時只讀不改。
	ManuallyEdited bool `json:"manuallyEdited,omitempty"`
	// OriginalText AI 校正修改此段時保存的校正前文字；空代表未被校正修改。
	OriginalText string `json:"originalText,omitempty"`
}

// UncertainRatio 回傳逐字稿中低信心字元（MarkedText 內「〔?…〕」包住者）佔總字元比例（0–1）。
// 以 Text 的 rune 數為分母；無段落或無文字回 0。
func UncertainRatio(segs []TranscriptSegment) float64 {
	total, uncertain := 0, 0
	for _, s := range segs {
		total += len([]rune(s.Text))
		in := false
		for _, r := range s.MarkedText {
			switch {
			case r == '〔':
				in = true
			case r == '〕':
				in = false
			case in && r != '?':
				uncertain++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(uncertain) / float64(total)
}

// AnalysisText 回傳供 LLM 分析用的段落文字：有低信心標記時用 MarkedText，否則用 Text。
func (s TranscriptSegment) AnalysisText() string {
	if s.MarkedText != "" {
		return s.MarkedText
	}
	return s.Text
}

// TranscribeResult STT 轉錄結果。
// Segments 為分講者片段（供應商支援 diarization 時填入）；不支援時為空，退回純 Text。
type TranscribeResult struct {
	Text            string
	Segments        []TranscriptSegment
	DurationSeconds int
}

// STTClient 語音轉文字 port（實作在 infrastructure/stt）。
// 實作自行處理供應商的檔案大小上限（必要時壓縮）；支援 diarization 者一併回填 Segments。
type STTClient interface {
	Transcribe(ctx context.Context, audio io.Reader, sizeBytes int64, filename string, language Language) (TranscribeResult, error)
}

// FlattenSegments 將分講者片段攤平為帶講者前綴的純文字（每段一行 "A: …"），
// 供既有 LLM 分析與關鍵字搜尋直接沿用。空片段、空文字略過；無講者代號時只留文字。
func FlattenSegments(segs []TranscriptSegment) string {
	lines := make([]string, 0, len(segs))
	for _, s := range segs {
		text := strings.TrimSpace(s.Text)
		if text == "" {
			continue
		}
		if s.Speaker == "" {
			lines = append(lines, text)
			continue
		}
		lines = append(lines, s.Speaker+": "+text)
	}
	return strings.Join(lines, "\n")
}

// TimestampedTranscript 將分講者片段組成帶時間錨點的逐字稿，供摘要 LLM 標註每個重點的音檔位置。
// 每行格式：[t=<起始毫秒>] <講者>: <內容>。內容用 AnalysisText（含低信心「〔?…〕」標記）。
// 無片段（未 diarize）時退回純逐字稿 fallback。
func TimestampedTranscript(segs []TranscriptSegment, fallback string) string {
	if len(segs) == 0 {
		return fallback
	}
	var b strings.Builder
	for _, s := range segs {
		if s.Speaker != "" {
			fmt.Fprintf(&b, "[t=%d] %s: %s\n", s.StartMs, s.Speaker, s.AnalysisText())
		} else {
			fmt.Fprintf(&b, "[t=%d] %s\n", s.StartMs, s.AnalysisText())
		}
	}
	return b.String()
}

// AnalysisTranscript 將分講者片段攤平成「講者: 內容」逐字稿，內容用 AnalysisText（含低信心標記）；
// 無片段時退回純逐字稿 fallback。
func AnalysisTranscript(segs []TranscriptSegment, fallback string) string {
	if len(segs) == 0 {
		return fallback
	}
	marked := make([]TranscriptSegment, len(segs))
	for i, s := range segs {
		marked[i] = TranscriptSegment{Speaker: s.Speaker, Text: s.AnalysisText()}
	}
	return FlattenSegments(marked)
}
