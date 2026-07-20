package search

import (
	"strings"
	"testing"
)

func TestSplitIntoChunks_ShortTextSingleChunk(t *testing.T) {
	got := SplitIntoChunks("這是一段很短的話。", 400, 1)
	if len(got) != 1 || got[0] != "這是一段很短的話。" {
		t.Fatalf("got %#v, want single chunk", got)
	}
}

func TestSplitIntoChunks_SplitsAtSentenceBoundaryNearTarget(t *testing.T) {
	// 每句約 50 字，target 100 → 每塊約 2 句
	s := ""
	for i := 0; i < 6; i++ {
		s += "這是一個大約有五十個字左右長度的測試句子用來驗證切塊邏輯是否正確運作良好。"
	}
	got := SplitIntoChunks(s, 100, 0)
	if len(got) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(got))
	}
	for _, c := range got {
		if len([]rune(c)) > 220 { // target 100 + 一句寬容
			t.Errorf("chunk too long (%d runes): %q", len([]rune(c)), c)
		}
	}
}

func TestSplitIntoChunks_OverlapCarriesLastSentence(t *testing.T) {
	s := "第一句話結束。第二句話結束。第三句話結束。第四句話結束。"
	got := SplitIntoChunks(s, 14, 1) // 每塊約 1 句，overlap 1 句
	if len(got) < 2 {
		t.Fatalf("expected multiple chunks, got %#v", got)
	}
	// 第二塊開頭應含第一塊最後一句
	if !strings.Contains(got[1], "第一句話結束。") && !strings.Contains(got[1], "第二句話結束。") {
		t.Errorf("chunk[1]=%q should overlap previous last sentence", got[1])
	}
}

func TestSplitIntoChunks_EmptyReturnsNil(t *testing.T) {
	if got := SplitIntoChunks("   ", 400, 1); len(got) != 0 {
		t.Fatalf("empty text should return no chunks, got %#v", got)
	}
}
