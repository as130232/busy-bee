package meeting

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// segmentEditorRepo 逐字稿片段編輯所需的最小 repo 介面（*db.MeetingRepo 滿足）。
type segmentEditorRepo interface {
	GetForUser(ctx context.Context, id, userID uuid.UUID) (domainmeeting.Meeting, error)
	UpdateTranscriptSegments(ctx context.Context, id, userID uuid.UUID, segments []domainmeeting.TranscriptSegment, transcript string) (domainmeeting.Meeting, error)
}

// EditSegmentUC 修正單一逐字稿片段的文字（校正 STT 錯字），並同步更新攤平 transcript。
type EditSegmentUC struct {
	repo segmentEditorRepo
}

func NewEditSegmentUC(repo segmentEditorRepo) *EditSegmentUC {
	return &EditSegmentUC{repo: repo}
}

// Execute 更新第 index 段的文字（本人限定）。text 去空白後不得為空；index 需在範圍內。
func (uc *EditSegmentUC) Execute(ctx context.Context, userID, meetingID uuid.UUID, index int, text string) (domainmeeting.Meeting, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return domainmeeting.Meeting{}, apperr.New(errcode.Param, "text")
	}
	m, err := uc.repo.GetForUser(ctx, meetingID, userID)
	if err != nil {
		if errors.Is(err, domainmeeting.ErrNotFound) {
			return domainmeeting.Meeting{}, apperr.New(errcode.NotFound)
		}
		return domainmeeting.Meeting{}, apperr.Wrap(err, errcode.Internal)
	}
	if index < 0 || index >= len(m.TranscriptSegments) {
		return domainmeeting.Meeting{}, apperr.New(errcode.Param, "index")
	}

	segs := m.TranscriptSegments
	segs[index].Text = text
	transcript := domainmeeting.FlattenSegments(segs)

	updated, err := uc.repo.UpdateTranscriptSegments(ctx, meetingID, userID, segs, transcript)
	if err != nil {
		if errors.Is(err, domainmeeting.ErrNotFound) {
			return domainmeeting.Meeting{}, apperr.New(errcode.NotFound)
		}
		return domainmeeting.Meeting{}, apperr.Wrap(err, errcode.Internal)
	}
	return updated, nil
}
