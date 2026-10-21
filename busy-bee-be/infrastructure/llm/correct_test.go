package llm

import (
	"errors"
	"strings"
	"testing"

	"github.com/as130232/busy-bee/busy-bee-be/domain/correction"
	"github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

func TestBuildCorrectionPrompt(t *testing.T) {
	in := correction.CorrectorInput{
		Context: []meeting.TranscriptSegment{{Speaker: "A", Text: "前文"}},
		Segments: []correction.IndexedSegment{
			{I: 7, Segment: meeting.TranscriptSegment{Speaker: "B", Text: "小娃處理", MarkedText: "〔?小娃〕處理", StartMs: 61000}},
			{I: 8, Segment: meeting.TranscriptSegment{Speaker: "A", Text: "人工"}, Locked: true},
		},
	}
	p := buildCorrectionPrompt(in)
	for _, want := range []string{"A: 前文", "[7] (01:01) B: 〔?小娃〕處理", "[8] 🔒 (00:00) A: 人工"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt 缺少 %q", want)
		}
	}
	if strings.Contains(p, "{{") {
		t.Error("prompt 有未替換的佔位符")
	}
}

func TestBuildCorrectionPrompt_NoContext(t *testing.T) {
	p := buildCorrectionPrompt(correction.CorrectorInput{Segments: []correction.IndexedSegment{{I: 0, Segment: meeting.TranscriptSegment{Speaker: "A", Text: "x"}}}})
	if !strings.Contains(p, "（無）") {
		t.Error("無上文時應填（無）")
	}
}

func TestParsePatches(t *testing.T) {
	got, err := parsePatches("```json\n[{\"i\":7,\"text\":\"小王處理\"}]\n```")
	if err != nil || len(got) != 1 || got[0].I != 7 || got[0].Text != "小王處理" {
		t.Errorf("got %+v err %v", got, err)
	}
	if _, err := parsePatches("not json"); !errors.Is(err, correction.ErrBadOutput) {
		t.Errorf("壞 JSON 應回 ErrBadOutput，got %v", err)
	}
}

// 審查 I2：片段音訊從 ClipStartMs 開始，prompt 時間需相對於片段起點，模型才能依時間對位。
func TestBuildCorrectionPrompt_TimesRelativeToClip(t *testing.T) {
	p := buildCorrectionPrompt(correction.CorrectorInput{
		ClipStartMs: 690_000, // 11:30
		Segments: []correction.IndexedSegment{
			{I: 3, Segment: meeting.TranscriptSegment{Speaker: "A", Text: "x", StartMs: 720_000}}, // 12:00 → 片段內 00:30
		},
	})
	if !strings.Contains(p, "[3] (00:30) A: x") {
		t.Errorf("時間應相對片段起點，prompt:\n%s", p)
	}
}

func TestBuildCorrectionPrompt_Background(t *testing.T) {
	in := correction.CorrectorInput{Segments: []correction.IndexedSegment{{I: 0, Segment: meeting.TranscriptSegment{Speaker: "A", Text: "史得anchor"}}}}
	plain := buildCorrectionPrompt(in)
	in.Background = "## 背景資料（會前提供，非會議內容）\n\n### 名詞\nStage"
	got := buildCorrectionPrompt(in)
	if !strings.HasPrefix(got, in.Background) || !strings.HasSuffix(got, plain) {
		t.Errorf("背景資料需放最前面、原 prompt 不變：%q", got[:60])
	}
}

// 使用者補充：錯字對照與說明附在 prompt 最後，並明示不得寫入音檔沒說的內容；沒有補充時 prompt 不變。
func TestBuildCorrectionPrompt_UserGuidance(t *testing.T) {
	in := correction.CorrectorInput{Segments: []correction.IndexedSegment{{I: 0, Segment: meeting.TranscriptSegment{Speaker: "A", Text: "DVA 報告"}}}}
	plain := buildCorrectionPrompt(in)
	in.Terms = []meeting.ReplaceRule{{From: "DBA", To: "DVA"}}
	in.Note = "DVA 是內部報告代號"
	got := buildCorrectionPrompt(in)
	if !strings.HasPrefix(got, plain) {
		t.Error("補充應附在原 prompt 之後")
	}
	for _, want := range []string{"DBA → DVA", "DVA 是內部報告代號", "不得"} {
		if !strings.Contains(got[len(plain):], want) {
			t.Errorf("補充段缺少 %q：%s", want, got[len(plain):])
		}
	}
}
