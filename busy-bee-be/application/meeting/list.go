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

// ListUC 本人會議列表（可帶關鍵字搜尋 title/transcript）。
type ListUC struct {
	repo domainmeeting.Repository
}

func NewListUC(repo domainmeeting.Repository) *ListUC {
	return &ListUC{repo: repo}
}

func (uc *ListUC) Execute(ctx context.Context, userID uuid.UUID, search string) ([]domainmeeting.Meeting, error) {
	list, err := uc.repo.ListForUser(ctx, userID, strings.TrimSpace(search))
	if err != nil {
		return nil, apperr.Wrap(err, errcode.Internal)
	}
	return list, nil
}

// GetUC 會議詳情（含 transcript）。
type GetUC struct {
	repo domainmeeting.Repository
}

func NewGetUC(repo domainmeeting.Repository) *GetUC {
	return &GetUC{repo: repo}
}

func (uc *GetUC) Execute(ctx context.Context, userID, meetingID uuid.UUID) (domainmeeting.Meeting, error) {
	m, err := uc.repo.GetForUser(ctx, meetingID, userID)
	if err != nil {
		if errors.Is(err, domainmeeting.ErrNotFound) {
			return domainmeeting.Meeting{}, apperr.New(errcode.NotFound)
		}
		return domainmeeting.Meeting{}, apperr.Wrap(err, errcode.Internal)
	}
	return m, nil
}
