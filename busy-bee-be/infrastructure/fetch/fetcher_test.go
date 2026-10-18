package fetch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/netguard"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/tempslot"
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

// 大檔暫存名額被占用時，Fetch 在下載前等待；ctx 到期即放棄，不落地任何檔案。
func TestFetch_WaitsForTempSlot(t *testing.T) {
	slots := tempslot.New(1)
	hold, _ := slots.Acquire(context.Background())
	defer hold()

	f := New(WithTempSlots(slots))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := f.Fetch(ctx, "https://203.0.113.10/ep.mp3"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
}

// 名額持有到 Reader.Close（暫存檔刪除）為止。
func TestFetch_ReleasesSlotOnClose(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("audio"))
	}))
	defer srv.Close()

	slots := tempslot.New(1)
	f := New(WithTempSlots(slots))
	f.http = srv.Client()
	f.checkHost = func(context.Context, string) error { return nil } // 測試 server 在 loopback

	got, err := f.Fetch(context.Background(), srv.URL+"/a.mp3")
	if err != nil {
		t.Fatal(err)
	}
	busy, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := slots.Acquire(busy); err == nil {
		t.Fatal("slot must be held until Close")
	}
	_ = got.Reader.Close()
	release, err := slots.Acquire(context.Background())
	if err != nil {
		t.Fatalf("slot not released after Close: %v", err)
	}
	release()
}
