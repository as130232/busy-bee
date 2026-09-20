package stt

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
	got, err := c.Transcribe(context.Background(), strings.NewReader("audio"), 5, "m.webm", "")
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
	_, err := c.Transcribe(context.Background(), strings.NewReader("x"), 1, "a.mp3", "")

	if err == nil || !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Fatalf("err = %v, want groq error message surfaced", err)
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
	got, err := c.Transcribe(context.Background(), strings.NewReader("x"), 1, "a.mp3", "")
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
	got, err := c.Transcribe(context.Background(), strings.NewReader("x"), 1, "a.mp3", "")
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if got.Text != "純文字回應" {
		t.Errorf("Text = %q", got.Text)
	}
}
