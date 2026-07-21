package meeting

import (
	"context"
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
	Transcribe(ctx context.Context, audio io.Reader, sizeBytes int64, filename string) (TranscribeResult, error)
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
