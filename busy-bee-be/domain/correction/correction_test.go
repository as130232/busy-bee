package correction

import (
	"testing"

	"github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

func TestStateFromLatest(t *testing.T) {
	cases := []struct {
		name   string
		latest *Correction
		count  int
		want   State
	}{
		{"無紀錄", nil, 0, State{}},
		{"完成可還原", &Correction{Status: StatusCompleted}, 1, State{Status: StatusCompleted, AttemptsUsed: 1, CanRevert: true}},
		{"已還原不可再還原", &Correction{Status: StatusReverted}, 2, State{Status: StatusReverted, AttemptsUsed: 2}},
		{"失敗帶訊息", &Correction{Status: StatusFailed, ErrorMessage: "x"}, 1, State{Status: StatusFailed, AttemptsUsed: 1, ErrorMessage: "x"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := StateFromLatest(c.latest, c.count); got != c.want {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestApplyPatches_SetsOriginalAndClearsMark(t *testing.T) {
	cur := []meeting.TranscriptSegment{{Speaker: "B", Text: "小娃處理", MarkedText: "〔?小娃〕處理", StartMs: 5}}
	got := ApplyPatches(cur, []Patch{{I: 0, Text: "小王處理"}})
	s := got[0]
	if s.Text != "小王處理" || s.OriginalText != "小娃處理" || s.MarkedText != "" || s.Speaker != "B" || s.StartMs != 5 {
		t.Errorf("got %+v", s)
	}
	if cur[0].Text != "小娃處理" {
		t.Error("不可修改輸入 slice")
	}
}

func TestApplyPatches_KeepsFirstOriginalText(t *testing.T) {
	cur := []meeting.TranscriptSegment{{Text: "小王處理", OriginalText: "小娃處理"}}
	got := ApplyPatches(cur, []Patch{{I: 0, Text: "小王來處理"}})
	if got[0].OriginalText != "小娃處理" {
		t.Errorf("OriginalText = %q，應保留最初原文", got[0].OriginalText)
	}
}

func TestApplyPatches_SkipsSegmentsEditedDuringCorrection(t *testing.T) {
	cur := []meeting.TranscriptSegment{{Text: "使用者剛改的", ManuallyEdited: true}}
	got := ApplyPatches(cur, []Patch{{I: 0, Text: "AI 版本"}})
	if got[0].Text != "使用者剛改的" || got[0].OriginalText != "" {
		t.Errorf("got %+v", got[0])
	}
}

func TestApplyPatches_IgnoresOutOfRange(t *testing.T) {
	got := ApplyPatches([]meeting.TranscriptSegment{{Text: "a"}}, []Patch{{I: 5, Text: "b"}, {I: -1, Text: "c"}})
	if got[0].Text != "a" {
		t.Errorf("got %+v", got)
	}
}

// 還原時保留「快照之後」使用者手動修改的段落（審查 I1：使用者修改的內容不被覆蓋）。
func TestRevertSegments_KeepsPostCorrectionManualEdits(t *testing.T) {
	snap := []meeting.TranscriptSegment{
		{Speaker: "A", Text: "小娃"},
		{Speaker: "B", Text: "原本就手改", ManuallyEdited: true},
		{Speaker: "A", Text: "眼角老"},
	}
	current := []meeting.TranscriptSegment{
		{Speaker: "A", Text: "小王", OriginalText: "小娃"},                              // AI 改的 → 還原
		{Speaker: "B", Text: "原本就手改", ManuallyEdited: true},                         // 未變 → 用快照
		{Speaker: "A", Text: "使用者校正後再改", ManuallyEdited: true, OriginalText: "眼角老"}, // 校正後手改 → 保留，清 OriginalText
	}
	got := RevertSegments(snap, current)
	if got[0].Text != "小娃" || got[0].OriginalText != "" {
		t.Errorf("seg0 = %+v", got[0])
	}
	if got[1].Text != "原本就手改" || !got[1].ManuallyEdited {
		t.Errorf("seg1 = %+v", got[1])
	}
	if got[2].Text != "使用者校正後再改" || !got[2].ManuallyEdited || got[2].OriginalText != "" {
		t.Errorf("seg2 = %+v", got[2])
	}
}

func TestRevertSegments_LengthMismatchFallsBackToSnapshot(t *testing.T) {
	snap := []meeting.TranscriptSegment{{Text: "a"}}
	got := RevertSegments(snap, []meeting.TranscriptSegment{{Text: "x", ManuallyEdited: true}, {Text: "y"}})
	if len(got) != 1 || got[0].Text != "a" {
		t.Errorf("got %+v", got)
	}
}
