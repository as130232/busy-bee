package actionitem

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	domainactionitem "github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// AddUC 手動新增待辦（owner-only：先驗會議所有權）。
type AddUC struct {
	meetings domainmeeting.Repository
	items    domainactionitem.Repository
}

func NewAddUC(meetings domainmeeting.Repository, items domainactionitem.Repository) *AddUC {
	return &AddUC{meetings: meetings, items: items}
}

func (uc *AddUC) Execute(ctx context.Context, userID, meetingID uuid.UUID, description, assignee string) (domainactionitem.ActionItem, error) {
	desc := strings.TrimSpace(description)
	if desc == "" {
		return domainactionitem.ActionItem{}, apperr.New(errcode.Param)
	}
	if _, err := uc.meetings.GetForUser(ctx, meetingID, userID); err != nil {
		if errors.Is(err, domainmeeting.ErrNotFound) {
			return domainactionitem.ActionItem{}, apperr.New(errcode.NotFound)
		}
		return domainactionitem.ActionItem{}, apperr.Wrap(err, errcode.Internal)
	}
	item, err := uc.items.InsertManual(ctx, meetingID, userID, desc, strings.TrimSpace(assignee))
	if err != nil {
		return domainactionitem.ActionItem{}, apperr.Wrap(err, errcode.Internal)
	}
	return item, nil
}
