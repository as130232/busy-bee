package meeting

import (
	"context"
	"errors"
	"log/slog"
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

// EditSegmentUC 修正單一逐字稿片段的文字（校正 STT 錯字），並同步更新攤平 transcript 與語意索引。
type EditSegmentUC struct {
	repo    segmentEditorRepo
	indexer MeetingIndexer // 選填；nil 時跳過重新索引
}

func NewEditSegmentUC(repo segmentEditorRepo, indexer MeetingIndexer) *EditSegmentUC {
	return &EditSegmentUC{repo: repo, indexer: indexer}
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

	// 修正後重建語意索引（best-effort；失敗不影響編輯結果，避免搜尋引用舊文字）。
	if uc.indexer != nil {
		if ierr := uc.indexer.Execute(ctx, meetingID); ierr != nil {
			slog.WarnContext(ctx, "meeting.edit_segment.reindex_failed", "meeting_id", meetingID, "err", ierr)
		}
	}
	return updated, nil
}
