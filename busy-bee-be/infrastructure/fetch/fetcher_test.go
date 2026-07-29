package fetch

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestFetch_InvalidURL(t *testing.T) {
	f := New("")
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

func TestCookieArgs_Empty(t *testing.T) {
	f := New("")
	args, cleanup := f.cookieArgs()
	defer cleanup()
	if args != nil {
		t.Errorf("expected no args when cookiesPath empty, got %v", args)
	}
}

func TestCookieArgs_MissingFile(t *testing.T) {
	// cookies 路徑設了但檔案不存在 → 退回不帶 cookies（不報錯）。
	f := New(filepath.Join(t.TempDir(), "nope.txt"))
	args, cleanup := f.cookieArgs()
	defer cleanup()
	if args != nil {
		t.Errorf("expected no args when cookies file missing, got %v", args)
	}
}

func TestCookieArgs_CopiesToWritableTemp(t *testing.T) {
	src := filepath.Join(t.TempDir(), "cookies.txt")
	if err := os.WriteFile(src, []byte("# Netscape HTTP Cookie File\n"), 0o400); err != nil {
		t.Fatal(err)
	}
	f := New(src)
	args, cleanup := f.cookieArgs()
	defer cleanup()
	if len(args) != 2 || args[0] != "--cookies" {
		t.Fatalf("expected [--cookies <path>], got %v", args)
	}
	// 副本須存在且可寫（yt-dlp 會寫回 cookiejar）。
	info, err := os.Stat(args[1])
	if err != nil {
		t.Fatalf("temp cookies file missing: %v", err)
	}
	if info.Mode().Perm()&0o200 == 0 {
		t.Errorf("temp cookies file not writable: %v", info.Mode())
	}
	cleanup()
	if _, err := os.Stat(args[1]); !os.IsNotExist(err) {
		t.Errorf("expected temp cookies file removed after cleanup")
	}
}

func TestLooksDirectAudio_ByExtension(t *testing.T) {
	f := New("")
	// 副檔名為音檔者短路判定為 true，不會發 HEAD 請求。
	u, _ := url.Parse("https://cdn.example.com/ep.mp3")
	if !f.looksDirectAudio(context.Background(), u) {
		t.Error("expected .mp3 to be direct audio")
	}
}
