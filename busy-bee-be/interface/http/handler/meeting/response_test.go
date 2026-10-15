package meeting

import (
	"testing"
	"time"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

func TestApplySpeakerNames(t *testing.T) {
	names := map[string]string{"A": "Ben", "B": "王小明"}

	cases := []struct {
		name    string
		snippet string
		want    string
	}{
		{"single line code replaced", "A: 我們來討論定價", "Ben: 我們來討論定價"},
		{"multi line each replaced", "A: 定價策略\nB: 我同意", "Ben: 定價策略\n王小明: 我同意"},
		{"unknown code untouched", "C: 未命名講者", "C: 未命名講者"},
		{"no speaker prefix untouched", "純文字沒有講者", "純文字沒有講者"},
		{"empty names untouched", "A: 定價", "A: 定價"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n := names
			if c.name == "empty names untouched" {
				n = nil
			}
			if got := applySpeakerNames(c.snippet, n); got != c.want {
				t.Errorf("applySpeakerNames(%q) = %q, want %q", c.snippet, got, c.want)
			}
		})
	}
}

func TestToMeetingDetailResponse_CorrectionFields(t *testing.T) {
	m := domainmeeting.Meeting{TranscriptSegments: []domainmeeting.TranscriptSegment{
		{Speaker: "A", Text: "好", MarkedText: "〔?好〕", ManuallyEdited: true, OriginalText: "號"},
	}}
	r := toMeetingDetailResponse(m)
	if !r.TranscriptSegments[0].ManuallyEdited || r.TranscriptSegments[0].OriginalText != "號" {
		t.Errorf("segment = %+v", r.TranscriptSegments[0])
	}
	if r.UncertainRatio != 1 {
		t.Errorf("UncertainRatio = %v, want 1", r.UncertainRatio)
	}
}

func TestToMeetingDetailResponse_LiveFields(t *testing.T) {
	started := time.Date(2026, 10, 15, 14, 0, 0, 0, time.UTC)
	m := domainmeeting.Meeting{Status: domainmeeting.StatusScheduled, LiveStartedAt: &started, LiveNotes: &domainmeeting.LiveNotes{
		Notes: domainmeeting.LiveSummary{Current: "c", Points: []string{"p"}},
		QA:    []domainmeeting.LiveQA{{Question: "q", Answer: "a", Sources: []domainmeeting.LiveSource{{Type: "transcript", Label: "14:03"}}}},
	}}
	r := toMeetingDetailResponse(m)
	if r.LiveStartedAt == nil || !r.LiveStartedAt.Equal(started) {
		t.Errorf("liveStartedAt = %v", r.LiveStartedAt)
	}
	if r.LiveNotes == nil || r.LiveNotes.Notes.Current != "c" || len(r.LiveNotes.QA) != 1 || r.LiveNotes.QA[0].Sources[0].Label != "14:03" {
		t.Errorf("liveNotes = %+v", r.LiveNotes)
	}
	if r := toMeetingDetailResponse(domainmeeting.Meeting{}); r.LiveNotes != nil || r.LiveStartedAt != nil {
		t.Errorf("非即時會議不應有 live 欄位：%+v", r)
	}
}
