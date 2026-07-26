//go:build integration

// 需要 ffmpeg 的壓縮整合測試：以 `go test -tags integration ./infrastructure/stt/...` 執行。
package stt

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func requireFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Fatal("ffmpeg required for integration tests but not installed")
	}
}

// genWAV 用 ffmpeg 產生 durationSec 秒的測試音訊 wav。
func genWAV(t *testing.T, durationSec int) string {
	t.Helper()
	requireFFmpeg(t)
	f := t.TempDir() + "/test.wav"
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i",
		"sine=frequency=440:duration="+itoa(durationSec), "-y", f)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gen wav: %v\n%s", err, out)
	}
	return f
}

func itoa(i int) string { return string(rune('0' + i)) }

func TestCompressToMP3_ShrinksAudio(t *testing.T) {
	wav := genWAV(t, 5)
	in, err := os.Open(wav)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	origInfo, _ := os.Stat(wav)

	out, size, err := CompressToMP3(context.Background(), in)
	if err != nil {
		t.Fatalf("CompressToMP3() error = %v", err)
	}
	defer out.Close()

	if size <= 0 || size >= origInfo.Size() {
		t.Errorf("compressed size = %d, want 0 < size < original %d", size, origInfo.Size())
	}
	head := make([]byte, 2)
	io.ReadFull(out, head)
	// mp3 開頭為 ID3 tag 或 frame sync (0xFF 0xFB/0xF3/0xF2)
	isMP3 := (head[0] == 'I' && head[1] == 'D') || head[0] == 0xFF
	if !isMP3 {
		t.Errorf("output does not look like mp3: % x", head)
	}
}

func TestTranscribe_OversizedTriggersCompression(t *testing.T) {
	requireFFmpeg(t)
	cap := &capturedRequest{}
	srv := fakeGroqServer(t, http.StatusOK, `{"text":"ok","duration":5}`, cap)

	wav := genWAV(t, 5)
	in, _ := os.Open(wav)
	defer in.Close()
	info, _ := os.Stat(wav)

	// 上限 50KB：原始 wav（約 220KB）觸發壓縮，壓縮後（約 10KB）可通過
	c := New("k", WithBaseURL(srv.URL), WithMaxUploadBytes(50*1024))
	_, err := c.Transcribe(context.Background(), in, info.Size(), "big.wav")
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}

	if !strings.HasSuffix(cap.filename, ".mp3") {
		t.Errorf("filename = %q, want compressed .mp3 sent", cap.filename)
	}
	if cap.fileSize >= int(info.Size()) {
		t.Errorf("sent %d bytes, want smaller than original %d", cap.fileSize, info.Size())
	}
}
