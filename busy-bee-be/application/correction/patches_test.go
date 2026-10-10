package correction

import (
	"reflect"
	"testing"

	"github.com/as130232/busy-bee/busy-bee-be/domain/correction"
	"github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

func TestValidatePatches(t *testing.T) {
	segs := []meeting.TranscriptSegment{
		{Text: "小娃處理"}, {Text: "接流串接", ManuallyEdited: true}, {Text: "今年預算"}, {Text: "外段"},
	}
	ch := correction.Chunk{First: 0, Last: 2}
	in := []correction.Patch{
		{I: 0, Text: "小王處理"},          // 保留
		{I: 1, Text: "金流串接"},          // 鎖定 → 丟
		{I: 2, Text: "今年預算"},          // 與原文相同 → 丟
		{I: 3, Text: "外段改"},           // 越界 → 丟
		{I: 0, Text: ""},               // 空 → 丟
		{I: 2, Text: "今年預算是三百萬元整加上其他費用"}, // 長度比 > 2 → 丟
		{I: -1, Text: "負索引"},          // 越界 → 丟
	}
	got := ValidatePatches(segs, ch, in)
	want := []correction.Patch{{I: 0, Text: "小王處理"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestApplyPatches_SetsOriginalAndClearsMark(t *testing.T) {
	cur := []meeting.TranscriptSegment{{Speaker: "B", Text: "小娃處理", MarkedText: "〔?小娃〕處理", StartMs: 5}}
	got := ApplyPatches(cur, []correction.Patch{{I: 0, Text: "小王處理"}})
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
	got := ApplyPatches(cur, []correction.Patch{{I: 0, Text: "小王來處理"}})
	if got[0].OriginalText != "小娃處理" {
		t.Errorf("OriginalText = %q，應保留最初原文", got[0].OriginalText)
	}
}

func TestApplyPatches_SkipsSegmentsEditedDuringCorrection(t *testing.T) {
	cur := []meeting.TranscriptSegment{{Text: "使用者剛改的", ManuallyEdited: true}}
	got := ApplyPatches(cur, []correction.Patch{{I: 0, Text: "AI 版本"}})
	if got[0].Text != "使用者剛改的" || got[0].OriginalText != "" {
		t.Errorf("got %+v", got[0])
	}
}

func TestApplyPatches_IgnoresOutOfRange(t *testing.T) {
	got := ApplyPatches([]meeting.TranscriptSegment{{Text: "a"}}, []correction.Patch{{I: 5, Text: "b"}, {I: -1, Text: "c"}})
	if got[0].Text != "a" {
		t.Errorf("got %+v", got)
	}
}
