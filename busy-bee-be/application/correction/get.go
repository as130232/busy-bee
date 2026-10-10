package correction

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/as130232/busy-bee/busy-bee-be/domain/correction"
	"github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// GetUC 查詢紀錄最新一次校正狀態（owner-only）。
type GetUC struct {
	meetings MeetingReader
	repo     correction.Repository
}

func NewGetUC(meetings MeetingReader, repo correction.Repository) *GetUC {
	return &GetUC{meetings: meetings, repo: repo}
}

func (uc *GetUC) Execute(ctx context.Context, userID, meetingID uuid.UUID) (correction.State, error) {
	if _, err := uc.meetings.GetForUser(ctx, meetingID, userID); err != nil {
		if errors.Is(err, meeting.ErrNotFound) {
			return correction.State{}, apperr.New(errcode.NotFound)
		}
		return correction.State{}, apperr.Wrap(err, errcode.Internal)
	}
	return latestState(ctx, uc.repo, meetingID, userID)
}

// latestState 最新一筆 + 總筆數 → 對外狀態；無紀錄回零值。
func latestState(ctx context.Context, repo correction.Repository, meetingID, userID uuid.UUID) (correction.State, error) {
	c, err := repo.GetLatestForMeeting(ctx, meetingID, userID)
	if errors.Is(err, correction.ErrNotFound) {
		return correction.State{}, nil
	}
	if err != nil {
		return correction.State{}, apperr.Wrap(err, errcode.Internal)
	}
	n, err := repo.CountForMeeting(ctx, meetingID, userID)
	if err != nil {
		return correction.State{}, apperr.Wrap(err, errcode.Internal)
	}
	return correction.StateFromLatest(&c, n), nil
}
