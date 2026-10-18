package fetch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/netguard"
)

func TestFetch_InvalidURL(t *testing.T) {
	f := New()
	for _, bad := range []string{"", "not a url", "ftp://x.com/a.mp3", "javascript:alert(1)", "/relative/a.mp3"} {
		if _, err := f.Fetch(context.Background(), bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestTitleFromURL(t *testing.T) {
	cases := map[string]string{
		"https://cdn.example.com/podcasts/ep-42.mp3": "ep-42",
		"https://x.com/a/b/hello.m4a":                "hello",
		"https://x.com/":                             "匯入的音訊",
	}
	for raw, want := range cases {
		u, _ := url.Parse(raw)
		if got := titleFromURL(u); got != want {
			t.Errorf("titleFromURL(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestLooksDirectAudio_ByExtension(t *testing.T) {
	f := New()
	// 副檔名為音檔者短路判定為 true，不會發 HEAD 請求。
	u, _ := url.Parse("https://cdn.example.com/ep.mp3")
	if !f.looksDirectAudio(context.Background(), u) {
		t.Error("expected .mp3 to be direct audio")
	}
}

// SSRF：直接音檔網址指向內網，下載必須被擋。
func TestFetch_BlocksPrivateDirectAudio(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("secret"))
	}))
	defer srv.Close()

	_, err := New().Fetch(context.Background(), srv.URL+"/a.mp3")
	if !errors.Is(err, netguard.ErrBlocked) {
		t.Fatalf("err = %v, want netguard.ErrBlocked", err)
	}
}

// SSRF：非音檔網址會交給 yt-dlp；指向內網時必須在呼叫 yt-dlp 前就擋下。
func TestFetch_BlocksPrivateBeforeYtdlp(t *testing.T) {
	f := New()
	f.ytdlpPath = "/nonexistent/yt-dlp-must-not-run"
	for _, u := range []string{"http://169.254.169.254/computeMetadata/v1/", "http://localhost:8080/page", "http://[::1]/x"} {
		_, err := f.Fetch(context.Background(), u)
		if !errors.Is(err, netguard.ErrBlocked) {
			t.Errorf("Fetch(%q) err = %v, want netguard.ErrBlocked", u, err)
		}
	}
}
