package actionitem

import (
	"testing"
	"time"
)

func TestExtractedDueDate(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*3600)

	t.Run("valid ISO 回上午9點 UTC+8", func(t *testing.T) {
		got := Extracted{DueISO: "2026-07-24"}.DueDate()
		if got == nil {
			t.Fatal("want non-nil due date")
		}
		want := time.Date(2026, 7, 24, 9, 0, 0, 0, loc)
		if !got.Equal(want) {
			t.Errorf("DueDate = %v, want %v", got, want)
		}
	})

	t.Run("空字串回 nil（不排提醒）", func(t *testing.T) {
		if got := (Extracted{DueISO: ""}).DueDate(); got != nil {
			t.Errorf("want nil, got %v", got)
		}
	})

	t.Run("格式錯誤回 nil", func(t *testing.T) {
		if got := (Extracted{DueISO: "下週五"}).DueDate(); got != nil {
			t.Errorf("want nil, got %v", got)
		}
	})

	t.Run("前後空白容忍", func(t *testing.T) {
		if got := (Extracted{DueISO: "  2026-01-02 "}).DueDate(); got == nil {
			t.Error("want non-nil for padded ISO date")
		}
	})
}
