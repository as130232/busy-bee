package meetingcontext

import (
	"slices"
	"strings"
	"testing"
)

func TestPromptBlock(t *testing.T) {
	if got := PromptBlock(nil); got != "" {
		t.Fatalf("PromptBlock(nil) = %q, want empty", got)
	}
	got := PromptBlock([]Context{{Title: "規格書.pdf", Content: " A 內容 \n"}, {Title: "貼上的文字", Content: "B 內容"}})
	if !strings.HasPrefix(got, "## 背景資料（會前提供，非會議內容）") {
		t.Errorf("missing header: %q", got)
	}
	ia, ib := strings.Index(got, "### 規格書.pdf\nA 內容"), strings.Index(got, "### 貼上的文字\nB 內容")
	if ia < 0 || ib < 0 || ia > ib {
		t.Errorf("titles/content order wrong: %q", got)
	}
}

func TestMergeKeyterms(t *testing.T) {
	cs := []Context{{Keyterms: []string{"Stage", " busy-bee ", "QA"}}, {Keyterms: []string{"stage", "", "Jenkins"}}}
	got := MergeKeyterms([]string{"QA", "Deepgram"}, cs, 100)
	want := []string{"QA", "Deepgram", "Stage", "busy-bee", "Jenkins"}
	if !slices.Equal(got, want) {
		t.Errorf("MergeKeyterms = %v, want %v", got, want)
	}
	if got := MergeKeyterms([]string{"G1", "G2"}, cs, 3); !slices.Equal(got, []string{"G1", "G2", "Stage"}) {
		t.Errorf("limit: got %v (global must win)", got)
	}
	if got := MergeKeyterms(nil, nil, 10); got == nil || len(got) != 0 {
		t.Errorf("empty input must return empty non-nil slice, got %#v", got)
	}
}
