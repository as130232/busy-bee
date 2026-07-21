package meeting

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

type fakeSegmentRepo struct {
	meeting   domainmeeting.Meeting
	savedSegs []domainmeeting.TranscriptSegment
	savedText string
}

func (f *fakeSegmentRepo) GetForUser(_ context.Context, _, _ uuid.UUID) (domainmeeting.Meeting, error) {
	return f.meeting, nil
}

func (f *fakeSegmentRepo) UpdateTranscriptSegments(_ context.Context, _, _ uuid.UUID, segments []domainmeeting.TranscriptSegment, transcript string) (domainmeeting.Meeting, error) {
	f.savedSegs = segments
	f.savedText = transcript
	m := f.meeting
	m.TranscriptSegments = segments
	m.Transcript = transcript
	return m, nil
}

func TestEditSegment_UpdatesTextAndReflattens(t *testing.T) {
	repo := &fakeSegmentRepo{meeting: domainmeeting.Meeting{
		TranscriptSegments: []domainmeeting.TranscriptSegment{
			{Speaker: "A", Text: "情緒教職"}, // 待修正
			{Speaker: "B", Text: "對啊"},
		},
	}}
	uc := NewEditSegmentUC(repo)

	m, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), 0, "  情緒價值  ")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repo.savedSegs[0].Text != "情緒價值" {
		t.Errorf("segment text = %q, want trimmed 情緒價值", repo.savedSegs[0].Text)
	}
	// 攤平文字應反映修正
	if repo.savedText != "A: 情緒價值\nB: 對啊" {
		t.Errorf("flattened = %q", repo.savedText)
	}
	if m.Transcript != repo.savedText {
		t.Errorf("returned transcript mismatch")
	}
}

func TestEditSegment_EmptyTextParamError(t *testing.T) {
	uc := NewEditSegmentUC(&fakeSegmentRepo{})
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), 0, "   ")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.Param {
		t.Fatalf("err = %v, want Param", err)
	}
}

func TestEditSegment_IndexOutOfRange(t *testing.T) {
	repo := &fakeSegmentRepo{meeting: domainmeeting.Meeting{
		TranscriptSegments: []domainmeeting.TranscriptSegment{{Speaker: "A", Text: "x"}},
	}}
	uc := NewEditSegmentUC(repo)
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), 5, "新文字")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.Param {
		t.Fatalf("err = %v, want Param (index 超範圍)", err)
	}
}
