package correction

import (
	"strings"
	"testing"

	"github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

func TestGuidance_Normalize(t *testing.T) {
	g, ok := Guidance{
		Replacements: []meeting.ReplaceRule{{From: " DBA ", To: "DVA"}, {From: "", To: ""}},
		Note:         "  會議在討論 DVA 報告  ",
	}.Normalize()
	if !ok {
		t.Fatal("want ok")
	}
	if len(g.Replacements) != 1 || g.Replacements[0] != (meeting.ReplaceRule{From: "DBA", To: "DVA"}) {
		t.Errorf("replacements = %+v（空白列應略過）", g.Replacements)
	}
	if g.Note != "會議在討論 DVA 報告" {
		t.Errorf("note = %q", g.Note)
	}
}

func TestGuidance_NormalizeRejects(t *testing.T) {
	tooMany := make([]meeting.ReplaceRule, MaxGuidanceRules+1)
	for i := range tooMany {
		tooMany[i] = meeting.ReplaceRule{From: "a" + strings.Repeat("x", i), To: "b"}
	}
	bad := []Guidance{
		{Replacements: []meeting.ReplaceRule{{From: "DBA", To: ""}}}, // 只填一邊
		{Replacements: []meeting.ReplaceRule{{From: "DBA", To: "DBA"}}},
		{Replacements: tooMany},
		{Note: strings.Repeat("字", MaxGuidanceNoteRunes+1)},
	}
	for i, g := range bad {
		if _, ok := g.Normalize(); ok {
			t.Errorf("case %d should be rejected", i)
		}
	}
}

func TestGuidance_ApplySkipsManuallyEdited(t *testing.T) {
	g := Guidance{Replacements: []meeting.ReplaceRule{{From: "DBA", To: "DVA"}, {From: "王曉明", To: "王小明"}}}
	segs := []meeting.TranscriptSegment{
		{Text: "DBA 由王曉明負責", MarkedText: "〔?DBA〕 由王曉明負責"},
		{Text: "DBA", ManuallyEdited: true},
		{Text: "好"},
	}
	out := g.Apply(segs)
	if out[0].Text != "DVA 由王小明負責" || out[0].MarkedText != "" {
		t.Errorf("seg0 = %+v", out[0])
	}
	if out[1].Text != "DBA" {
		t.Error("人工修正過的段落不套用")
	}
	if out[2].Text != "好" || segs[0].Text != "DBA 由王曉明負責" {
		t.Error("不應修改輸入")
	}
	if g.ApplyText("DBA 與 王曉明") != "DVA 與 王小明" {
		t.Error("ApplyText")
	}
}
