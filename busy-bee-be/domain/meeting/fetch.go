package meeting

import (
	"context"
	"io"
)

// FetchedAudio 從外部連結抓取到的音訊：Reader 為音檔內容（caller 負責 Close，
// 實作可於 Close 時清理暫存檔）；Title 為抓到的來源標題（供自動命名），可為空。
type FetchedAudio struct {
	Reader      io.ReadCloser
	Size        int64
	ContentType string
	Title       string
	DurationSec int
}

// AudioFetcher 從外部 URL 取得音訊 port（infrastructure/fetch 實作：
// 直接音檔/Podcast 走 HTTP，YouTube/其他走 yt-dlp）。實作需自行套用時長/大小上限。
type AudioFetcher interface {
	Fetch(ctx context.Context, url string) (FetchedAudio, error)
}
