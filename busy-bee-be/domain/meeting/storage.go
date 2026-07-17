package meeting

import "context"

// UploadTarget 前端直傳所需的目標：signed URL 與必帶 headers。
type UploadTarget struct {
	URL     string
	Headers map[string]string
}

// AudioStorage 音訊儲存 port（GCS 實作在 infrastructure/gcs）。
type AudioStorage interface {
	// SignedUploadURL 產生限定 content-type 與大小上限的直傳 URL。
	SignedUploadURL(ctx context.Context, objectPath, contentType string, maxBytes int64) (UploadTarget, error)
	// Exists 檢查物件是否已上傳。
	Exists(ctx context.Context, objectPath string) (bool, error)
}
