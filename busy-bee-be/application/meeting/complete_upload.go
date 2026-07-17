package meeting

import (
	"context"
	"errors"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// CompleteUploadUC 驗證音訊已上傳後將會議推進為 pending 並 enqueue 背景處理。
// 對已在處理中/已完成的會議冪等（直接回現況，不重複 enqueue）。
type CompleteUploadUC struct {
	repo    domainmeeting.Repository
	storage domainmeeting.AudioStorage
	queue   domainmeeting.TaskQueue
}

func NewCompleteUploadUC(repo domainmeeting.Repository, storage domainmeeting.AudioStorage, queue domainmeeting.TaskQueue) *CompleteUploadUC {
	return &CompleteUploadUC{repo: repo, storage: storage, queue: queue}
}

func (uc *CompleteUploadUC) Execute(ctx context.Context, userID, meetingID uuid.UUID) (domainmeeting.Meeting, error) {
	m, err := uc.repo.GetForUser(ctx, meetingID, userID)
	if err != nil {
		if errors.Is(err, domainmeeting.ErrNotFound) {
			return domainmeeting.Meeting{}, apperr.New(errcode.NotFound)
		}
		return domainmeeting.Meeting{}, apperr.Wrap(err, errcode.Internal)
	}

	// 冪等：已離開 scheduled 的會議代表 complete-upload 已被處理過
	if m.Status != domainmeeting.StatusScheduled {
		return m, nil
	}

	exists, err := uc.storage.Exists(ctx, m.AudioGCSPath)
	if err != nil {
		return domainmeeting.Meeting{}, apperr.Wrap(err, errcode.Internal)
	}
	if !exists {
		return domainmeeting.Meeting{}, apperr.New(errcode.Param, "audio not uploaded")
	}

	m, err = uc.repo.UpdateStatus(ctx, m.ID, domainmeeting.StatusScheduled, domainmeeting.StatusPending)
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
