package stt

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/as130232/busy-bee/busy-bee-be/domain/correction"
)

// FFmpegClipper 以 ffmpeg 剪出音訊片段（16kbps mono mp3，與 CompressToMP3 同規格）。
type FFmpegClipper struct{}

var _ correction.ClipCutter = FFmpegClipper{}

// CutClip 剪出 [fromMs, toMs] 片段到暫存檔；caller 用完呼叫 cleanup 刪除。
func (FFmpegClipper) CutClip(ctx context.Context, srcPath string, fromMs, toMs int) (string, func(), error) {
	dir, err := os.MkdirTemp("", "bb-clip-*")
	if err != nil {
		return "", nil, fmt.Errorf("clip mkdtemp: %w", err)
	}
	cleanup := func() { os.RemoveAll(dir) }
	out := filepath.Join(dir, "clip.mp3")
	cmd := exec.CommandContext(ctx, "ffmpeg", "-ss", msToSec(fromMs), "-to", msToSec(toMs),
		"-i", srcPath, "-ac", "1", "-b:a", "16k", "-y", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("clip ffmpeg: %w: %s", err, tail(b, 300))
	}
	return out, cleanup, nil
}

func msToSec(ms int) string { return strconv.FormatFloat(float64(ms)/1000, 'f', 3, 64) }

// CompressToTempFile 同 CompressToMP3，但回傳暫存檔路徑（供 ffmpeg 再剪片段）；caller 呼叫 cleanup 刪除。
func CompressToTempFile(ctx context.Context, audio io.Reader) (string, func(), error) {
	r, _, err := CompressToMP3(ctx, audio)
	if err != nil {
		return "", nil, err
	}
	// 只需路徑：關閉檔案描述子但保留暫存目錄，cleanup 時才刪除
	if err := r.File.Close(); err != nil {
		r.cleanup()
		return "", nil, fmt.Errorf("compress close: %w", err)
	}
	return r.File.Name(), r.cleanup, nil
}
