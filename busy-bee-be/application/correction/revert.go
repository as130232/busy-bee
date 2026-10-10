package correction

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"github.com/as130232/busy-bee/busy-bee-be/domain/correction"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// RevertUC 整份還原最新一次已完成的校正（逐字稿/摘要回到快照；行動項不動）。
type RevertUC struct {
	repo    correction.Repository
	indexer Indexer
}

func NewRevertUC(repo correction.Repository, indexer Indexer) *RevertUC {
	return &RevertUC{repo: repo, indexer: indexer}
}

func (uc *RevertUC) Execute(ctx context.Context, userID, meetingID uuid.UUID) (correction.State, error) {
	// GetLatestForMeeting 以 userID 過濾，兼作 owner 檢查
	c, err := uc.repo.GetLatestForMeeting(ctx, meetingID, userID)
	if errors.Is(err, correction.ErrNotFound) || (err == nil && c.Status != correction.StatusCompleted) {
		return correction.State{}, apperr.New(errcode.NothingToRevert)
	}
	if err != nil {
		return correction.State{}, apperr.Wrap(err, errcode.Internal)
	}
	if err := uc.repo.Revert(ctx, c); err != nil {
		if errors.Is(err, correction.ErrStatusConflict) {
			return correction.State{}, apperr.New(errcode.NothingToRevert)
		}
		return correction.State{}, apperr.Wrap(err, errcode.Internal)
	}
	if err := uc.indexer.Execute(ctx, meetingID); err != nil {
		slog.WarnContext(ctx, "correction.revert.reindex_failed", "meeting_id", meetingID, "err", err) // 回填掃描會補
	}
	c.Status = correction.StatusReverted
	n, err := uc.repo.CountForMeeting(ctx, meetingID, userID)
	if err != nil {
		return correction.State{}, apperr.Wrap(err, errcode.Internal)
	}
	return correction.StateFromLatest(&c, n), nil
}
