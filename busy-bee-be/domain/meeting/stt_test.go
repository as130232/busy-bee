package meeting

import "testing"

func TestFlattenSegments(t *testing.T) {
	tests := []struct {
		name string
		segs []TranscriptSegment
		want string
	}{
		{
			name: "empty",
			segs: nil,
			want: "",
		},
		{
			name: "multiple speakers each on its own prefixed line",
			segs: []TranscriptSegment{
				{Speaker: "A", Text: "我們先討論架構"},
				{Speaker: "B", Text: "好，我覺得用 Clean Architecture"},
				{Speaker: "A", Text: "同意"},
			},
			want: "A: 我們先討論架構\nB: 好，我覺得用 Clean Architecture\nA: 同意",
		},
		{
			name: "trims text and skips empty segments",
			segs: []TranscriptSegment{
				{Speaker: "A", Text: "  第一句  "},
				{Speaker: "B", Text: "   "},
				{Speaker: "A", Text: "第三句"},
			},
			want: "A: 第一句\nA: 第三句",
		},
		{
			name: "no speaker code falls back to plain text",
			segs: []TranscriptSegment{
				{Speaker: "", Text: "沒有講者資訊"},
			},
			want: "沒有講者資訊",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FlattenSegments(tt.segs); got != tt.want {
				t.Errorf("FlattenSegments() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestUncertainRatio(t *testing.T) {
	segs := []TranscriptSegment{
		{Text: "預算三百萬", MarkedText: "預算〔?三百萬〕"}, // 3/5 不確定
		{Text: "好的好的好"},                       // 0/5
	}
	if got := UncertainRatio(segs); got < 0.299 || got > 0.301 {
		t.Errorf("UncertainRatio = %v, want 0.3", got)
	}
	if got := UncertainRatio(nil); got != 0 {
		t.Errorf("nil = %v, want 0", got)
	}
}
