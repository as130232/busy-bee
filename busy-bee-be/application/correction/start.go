package correction

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"github.com/as130232/busy-bee/busy-bee-be/domain/correction"
	"github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// StartUC 建立 AI 校正：驗證可校正 → 原子寫快照與 queued 紀錄 → 排入校正佇列。
type StartUC struct {
	meetings MeetingReader
	repo     correction.Repository
	queue    Queue
}

func NewStartUC(meetings MeetingReader, repo correction.Repository, queue Queue) *StartUC {
	return &StartUC{meetings: meetings, repo: repo, queue: queue}
}

func (uc *StartUC) Execute(ctx context.Context, userID, meetingID uuid.UUID) (correction.State, error) {
	m, err := uc.meetings.GetForUser(ctx, meetingID, userID)
	if err != nil {
		if errors.Is(err, meeting.ErrNotFound) {
			return correction.State{}, apperr.New(errcode.NotFound)
		}
		return correction.State{}, apperr.Wrap(err, errcode.Internal)
	}
	if m.Status != meeting.StatusCompleted || m.AudioGCSPath == "" || len(m.TranscriptSegments) == 0 {
		return correction.State{}, apperr.New(errcode.CorrectionNotAllowed)
	}
	c, err := uc.repo.CreateWithSnapshot(ctx, m.ID, userID, correction.Snapshot{
		Segments: m.TranscriptSegments, Transcript: m.Transcript,
		SummarySections: m.SummarySections, Summary: m.Summary,
	})
	switch {
	case errors.Is(err, correction.ErrStatusConflict):
		return correction.State{}, apperr.New(errcode.CorrectionLimitReached)
	case errors.Is(err, correction.ErrActiveExists):
		return correction.State{}, apperr.New(errcode.CorrectionInProgress)
	case err != nil:
		return correction.State{}, apperr.Wrap(err, errcode.Internal)
	}
	if err := uc.queue.EnqueueCorrection(ctx, c.ID); err != nil {
		// 已落庫為 queued，sweeper 會補排；不回錯（使用者重按會被 InProgress 擋下）
		slog.WarnContext(ctx, "correction.start.enqueue_failed", "correction_id", c.ID, "err", err)
	}
	return correction.StateFromLatest(&c, c.Attempt), nil
}
