package stt

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func fakeGroqServer(t *testing.T, status int, body string, capture *capturedRequest) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if capture != nil {
			capture.auth = r.Header.Get("Authorization")
			if err := r.ParseMultipartForm(64 << 20); err != nil {
				t.Errorf("parse multipart: %v", err)
			}
			capture.model = r.FormValue("model")
			capture.responseFormat = r.FormValue("response_format")
			if f, fh, err := r.FormFile("file"); err == nil {
				capture.filename = fh.Filename
				b, _ := io.ReadAll(f)
				capture.fileSize = len(b)
				f.Close()
			}
		}
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

type capturedRequest struct {
	auth           string
	model          string
	responseFormat string
	filename       string
	fileSize       int
}

func TestTranscribe_SendsMultipartAndParsesResponse(t *testing.T) {
	cap := &capturedRequest{}
	resp, _ := json.Marshal(map[string]any{"text": "大家好，今天討論架構。", "duration": 12.7})
	srv := fakeGroqServer(t, http.StatusOK, string(resp), cap)

	c := New("test-key", WithBaseURL(srv.URL), WithMaxUploadBytes(1024))
	got, err := c.Transcribe(context.Background(), strings.NewReader("audio"), 5, "m.webm")
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}

	if got.Text != "大家好，今天討論架構。" {
		t.Errorf("Text = %q", got.Text)
	}
	if got.DurationSeconds != 13 {
		t.Errorf("DurationSeconds = %d, want 13 (rounded)", got.DurationSeconds)
	}
	if cap.auth != "Bearer test-key" {
		t.Errorf("auth = %q", cap.auth)
	}
	if cap.model != "whisper-large-v3" {
		t.Errorf("model = %q", cap.model)
	}
	if cap.responseFormat != "verbose_json" {
		t.Errorf("response_format = %q", cap.responseFormat)
	}
	if cap.filename != "m.webm" {
		t.Errorf("filename = %q", cap.filename)
	}
}

func TestTranscribe_APIErrorSurfacesMessage(t *testing.T) {
	srv := fakeGroqServer(t, http.StatusTooManyRequests,
		`{"error":{"message":"rate limit exceeded"}}`, nil)

	c := New("k", WithBaseURL(srv.URL), WithMaxUploadBytes(1024))
	_, err := c.Transcribe(context.Background(), strings.NewReader("x"), 1, "a.mp3")

	if err == nil || !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Fatalf("err = %v, want groq error message surfaced", err)
	}
}

func requireFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
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

func TestTranscribe_DedupesConsecutiveRepeatedSegments(t *testing.T) {
	resp, _ := json.Marshal(map[string]any{
		"text":     "ignored when segments present",
		"duration": 61.0,
		"segments": []map[string]any{
			{"text": " 大家好，歡迎來到一分鐘學人工智慧"},
			{"text": " 一分鐘學人工智慧"},          // 上一段尾部重複幻覺（子串）→ 應去除
			{"text": " 今天我們要用一句話說明什麼是人工智慧"},
			{"text": " 今天我們要用一句話說明什麼是人工智慧"}, // 完全重複 → 應去除
			{"text": " 人工智慧簡稱AI"},
		},
	})
	srv := fakeGroqServer(t, http.StatusOK, string(resp), nil)

	c := New("k", WithBaseURL(srv.URL), WithMaxUploadBytes(1<<20))
	got, err := c.Transcribe(context.Background(), strings.NewReader("x"), 1, "a.mp3")
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}

	want := "大家好，歡迎來到一分鐘學人工智慧 今天我們要用一句話說明什麼是人工智慧 人工智慧簡稱AI"
	if got.Text != want {
		t.Errorf("Text = %q\nwant   %q", got.Text, want)
	}
}

func TestTranscribe_NoSegmentsFallsBackToText(t *testing.T) {
	resp, _ := json.Marshal(map[string]any{"text": "純文字回應", "duration": 3.0})
	srv := fakeGroqServer(t, http.StatusOK, string(resp), nil)

	c := New("k", WithBaseURL(srv.URL), WithMaxUploadBytes(1<<20))
	got, err := c.Transcribe(context.Background(), strings.NewReader("x"), 1, "a.mp3")
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if got.Text != "純文字回應" {
		t.Errorf("Text = %q", got.Text)
	}
}
