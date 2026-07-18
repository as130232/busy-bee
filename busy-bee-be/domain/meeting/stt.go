package meeting

import (
	"context"
	"io"
)

// TranscribeResult STT 轉錄結果。
type TranscribeResult struct {
	Text            string
	DurationSeconds int
}

// STTClient 語音轉文字 port（Groq Whisper 實作在 infrastructure/stt）。
// 實作自行處理供應商的檔案大小上限（必要時壓縮）。
type STTClient interface {
	Transcribe(ctx context.Context, audio io.Reader, sizeBytes int64, filename string) (TranscribeResult, error)
}
