package meeting

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	domainactionitem "github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

type fakeReplaceRepo struct {
	fakeSegmentRepo
	segWrites     int
	savedSummary  *string
	savedSections []domainmeeting.SummarySection
}

func (f *fakeReplaceRepo) UpdateTranscriptSegments(ctx context.Context, id, userID uuid.UUID, segs []domainmeeting.TranscriptSegment, transcript string) (domainmeeting.Meeting, error) {
	f.segWrites++
	m, err := f.fakeSegmentRepo.UpdateTranscriptSegments(ctx, id, userID, segs, transcript)
	f.meeting = m
	return m, err
}

func (f *fakeReplaceRepo) SaveSummary(_ context.Context, _, _ uuid.UUID, summary string) (domainmeeting.Meeting, error) {
	f.savedSummary = &summary
	f.meeting.Summary = summary
	return f.meeting, nil
}

func (f *fakeReplaceRepo) SaveSummarySections(_ context.Context, _, _ uuid.UUID, sections []domainmeeting.SummarySection) (domainmeeting.Meeting, error) {
	f.savedSections = sections
	f.meeting.SummarySections = sections
	return f.meeting, nil
}

type fakeReplaceItems struct {
	items   []domainactionitem.ActionItem
	updated map[uuid.UUID]string
}

func (f *fakeReplaceItems) ListByMeeting(_ context.Context, _ uuid.UUID) ([]domainactionitem.ActionItem, error) {
	return f.items, nil
}

func (f *fakeReplaceItems) UpdateDescription(_ context.Context, id, _ uuid.UUID, d string) (domainactionitem.ActionItem, error) {
	if f.updated == nil {
		f.updated = map[uuid.UUID]string{}
	}
	f.updated[id] = d
	return domainactionitem.ActionItem{ID: id, Description: d}, nil
}

func replaceFixture() (*fakeReplaceRepo, *fakeReplaceItems, uuid.UUID) {
	itemID := uuid.New()
	repo := &fakeReplaceRepo{fakeSegmentRepo: fakeSegmentRepo{meeting: domainmeeting.Meeting{
		Status:  domainmeeting.StatusCompleted,
		Summary: "討論 DBA 報告",
		SummarySections: []domainmeeting.SummarySection{
			{Type: "decisions", Title: "決策", Items: []domainmeeting.SummaryPoint{{Text: "DBA 下週交", Heading: "DBA"}}},
		},
		TranscriptSegments: []domainmeeting.TranscriptSegment{
			{Speaker: "A", Text: "DBA 報告", MarkedText: "〔?DBA〕 報告"},
			{Speaker: "B", Text: "好"},
			{Speaker: "A", Text: "DBAs 不算，DBA 才算"},
		},
	}}}
	items := &fakeReplaceItems{items: []domainactionitem.ActionItem{
		{ID: itemID, Description: "整理 DBA 報告"},
		{ID: uuid.New(), Description: "訂會議室"},
	}}
	return repo, items, itemID
}

func TestReplaceText_TranscriptSummaryAndActionItems(t *testing.T) {
	repo, items, itemID := replaceFixture()
	uc := NewReplaceTextUC(repo, items, nil)

	res, err := uc.Execute(context.Background(), uuid.New(), uuid.New(),
		ReplaceTextInput{Rule: domainmeeting.ReplaceRule{From: " DBA ", To: "DVA"}, IncludeSummary: true})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if res.Counts != (ReplaceCounts{Transcript: 2, Summary: 3, ActionItems: 1}) {
		t.Errorf("counts = %+v", res.Counts)
	}
	segs := repo.savedSegs
	if segs[0].Text != "DVA 報告" || !segs[0].ManuallyEdited || segs[0].MarkedText != "" {
		t.Errorf("seg0 = %+v, want replaced + ManuallyEdited + MarkedText cleared", segs[0])
	}
	if segs[1].ManuallyEdited {
		t.Error("未變動的片段不應標記人工修正（AI 校正仍可處理）")
	}
	if segs[2].Text != "DBAs 不算，DVA 才算" {
		t.Errorf("seg2 = %q", segs[2].Text)
	}
	if repo.savedText != "A: DVA 報告\nB: 好\nA: DBAs 不算，DVA 才算" {
		t.Errorf("flattened = %q", repo.savedText)
	}
	if repo.savedSummary == nil || *repo.savedSummary != "討論 DVA 報告" {
		t.Errorf("summary = %v", repo.savedSummary)
	}
	if p := repo.savedSections[0].Items[0]; p.Text != "DVA 下週交" || p.Heading != "DVA" {
		t.Errorf("section item = %+v", p)
	}
	if len(items.updated) != 1 || items.updated[itemID] != "整理 DVA 報告" {
		t.Errorf("action items updated = %v", items.updated)
	}
}

func TestReplaceText_TranscriptOnly(t *testing.T) {
	repo, items, _ := replaceFixture()
	uc := NewReplaceTextUC(repo, items, nil)

	res, err := uc.Execute(context.Background(), uuid.New(), uuid.New(),
		ReplaceTextInput{Rule: domainmeeting.ReplaceRule{From: "DBA", To: "DVA"}})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if res.Counts != (ReplaceCounts{Transcript: 2}) {
		t.Errorf("counts = %+v", res.Counts)
	}
	if repo.savedSummary != nil || repo.savedSections != nil || len(items.updated) != 0 {
		t.Error("未勾選時不應動摘要與行動項")
	}
}

func TestReplaceText_DryRunCountsWithoutWriting(t *testing.T) {
	repo, items, _ := replaceFixture()
	uc := NewReplaceTextUC(repo, items, nil)

	res, err := uc.Execute(context.Background(), uuid.New(), uuid.New(),
		ReplaceTextInput{Rule: domainmeeting.ReplaceRule{From: "DBA", To: "DVA"}, IncludeSummary: true, DryRun: true})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if res.Counts != (ReplaceCounts{Transcript: 2, Summary: 3, ActionItems: 1}) {
		t.Errorf("counts = %+v", res.Counts)
	}
	if repo.segWrites != 0 || repo.savedSummary != nil || repo.savedSections != nil || len(items.updated) != 0 {
		t.Error("預覽不得寫入")
	}
}

func TestReplaceText_NoMatchSkipsWrites(t *testing.T) {
	repo, items, _ := replaceFixture()
	uc := NewReplaceTextUC(repo, items, nil)

	res, err := uc.Execute(context.Background(), uuid.New(), uuid.New(),
		ReplaceTextInput{Rule: domainmeeting.ReplaceRule{From: "XYZ", To: "ABC"}, IncludeSummary: true})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if res.Counts != (ReplaceCounts{}) || repo.segWrites != 0 || repo.savedSummary != nil {
		t.Errorf("無符合時不應寫入, counts = %+v", res.Counts)
	}
}

func TestReplaceText_InvalidRuleParamError(t *testing.T) {
	repo, items, _ := replaceFixture()
	uc := NewReplaceTextUC(repo, items, nil)
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(),
		ReplaceTextInput{Rule: domainmeeting.ReplaceRule{From: "DBA", To: " "}})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.Param {
		t.Fatalf("err = %v, want Param", err)
	}
}

func TestReplaceText_NotCompletedConflict(t *testing.T) {
	repo, items, _ := replaceFixture()
	repo.meeting.Status = domainmeeting.StatusAnalyzing
	uc := NewReplaceTextUC(repo, items, nil)
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(),
		ReplaceTextInput{Rule: domainmeeting.ReplaceRule{From: "DBA", To: "DVA"}})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.Conflict {
		t.Fatalf("err = %v, want Conflict", err)
	}
}

// 沒有分講者片段的舊紀錄：直接取代攤平逐字稿。
func TestReplaceText_PlainTranscriptWithoutSegments(t *testing.T) {
	repo := &fakeReplaceRepo{fakeSegmentRepo: fakeSegmentRepo{meeting: domainmeeting.Meeting{
		Status: domainmeeting.StatusCompleted, Transcript: "DBA 報告 DBA",
	}}}
	uc := NewReplaceTextUC(repo, &fakeReplaceItems{}, nil)
	res, err := uc.Execute(context.Background(), uuid.New(), uuid.New(),
		ReplaceTextInput{Rule: domainmeeting.ReplaceRule{From: "DBA", To: "DVA"}})
	if err != nil || res.Counts.Transcript != 2 || repo.savedText != "DVA 報告 DVA" {
		t.Errorf("res = %+v err = %v saved = %q", res, err, repo.savedText)
	}
}
