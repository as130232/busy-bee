package meeting

import (
	"context"
	"errors"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// RetryUC 手動重跑失敗的會議：failed → pending 並重新排入佇列。
type RetryUC struct {
	repo  domainmeeting.Repository
	queue domainmeeting.TaskQueue
}

func NewRetryUC(repo domainmeeting.Repository, queue domainmeeting.TaskQueue) *RetryUC {
	return &RetryUC{repo: repo, queue: queue}
}

func (uc *RetryUC) Execute(ctx context.Context, userID, meetingID uuid.UUID) (domainmeeting.Meeting, error) {
	m, err := uc.repo.GetForUser(ctx, meetingID, userID)
	if err != nil {
		if errors.Is(err, domainmeeting.ErrNotFound) {
			return domainmeeting.Meeting{}, apperr.New(errcode.NotFound)
		}
		return domainmeeting.Meeting{}, apperr.Wrap(err, errcode.Internal)
	}
	if m.Status != domainmeeting.StatusFailed {
		return domainmeeting.Meeting{}, apperr.New(errcode.Conflict)
	}

	m, err = uc.repo.UpdateStatus(ctx, m.ID, domainmeeting.StatusFailed, domainmeeting.StatusPending)
	if err != nil {
		if errors.Is(err, domainmeeting.ErrStatusConflict) {
			return domainmeeting.Meeting{}, apperr.New(errcode.Conflict)
		}
		return domainmeeting.Meeting{}, apperr.Wrap(err, errcode.Internal)
	}
	if err := uc.queue.EnqueueProcessMeeting(ctx, m.ID); err != nil {
		return domainmeeting.Meeting{}, apperr.Wrap(err, errcode.Internal)
	}
	return m, nil
}
