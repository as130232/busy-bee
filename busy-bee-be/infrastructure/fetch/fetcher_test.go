package fetch

import (
	"context"
	"net/url"
	"testing"
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
