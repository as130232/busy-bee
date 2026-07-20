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

// ManageUC 會議管理：重新命名（任何狀態）、刪除排程會議。
type ManageUC struct {
	repo domainmeeting.ManageRepository
}

func NewManageUC(repo domainmeeting.ManageRepository) *ManageUC {
	return &ManageUC{repo: repo}
}

func (uc *ManageUC) Rename(ctx context.Context, userID, meetingID uuid.UUID, title string) (domainmeeting.Meeting, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return domainmeeting.Meeting{}, apperr.New(errcode.Param, "title")
	}
	m, err := uc.repo.Rename(ctx, meetingID, userID, title)
	if err != nil {
		if errors.Is(err, domainmeeting.ErrNotFound) {
			return domainmeeting.Meeting{}, apperr.New(errcode.NotFound)
		}
		return domainmeeting.Meeting{}, apperr.Wrap(err, errcode.Internal)
	}
	return m, nil
}

func (uc *ManageUC) DeleteScheduled(ctx context.Context, userID, meetingID uuid.UUID) error {
	if err := uc.repo.DeleteScheduled(ctx, meetingID, userID); err != nil {
		if errors.Is(err, domainmeeting.ErrNotFound) {
			return apperr.New(errcode.NotFound)
		}
		return apperr.Wrap(err, errcode.Internal)
	}
	return nil
}
