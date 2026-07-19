// Package actionitem 提供行動項的 use cases（查詢與勾選）。
package actionitem

import (
	"context"
	"errors"

	"github.com/google/uuid"

	domainactionitem "github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// ListByMeetingUC 取回某會議的行動項（owner-only：先驗會議所有權）。
type ListByMeetingUC struct {
	meetings domainmeeting.Repository
	items    domainactionitem.Repository
}

func NewListByMeetingUC(meetings domainmeeting.Repository, items domainactionitem.Repository) *ListByMeetingUC {
	return &ListByMeetingUC{meetings: meetings, items: items}
}

func (uc *ListByMeetingUC) Execute(ctx context.Context, userID, meetingID uuid.UUID) ([]domainactionitem.ActionItem, error) {
	if _, err := uc.meetings.GetForUser(ctx, meetingID, userID); err != nil {
		if errors.Is(err, domainmeeting.ErrNotFound) {
			return nil, apperr.New(errcode.NotFound)
		}
		return nil, apperr.Wrap(err, errcode.Internal)
	}
	list, err := uc.items.ListByMeeting(ctx, meetingID)
	if err != nil {
		return nil, apperr.Wrap(err, errcode.Internal)
	}
	return list, nil
}

// ListPendingUC 取回本人跨會議的未完成行動項。
type ListPendingUC struct {
	items domainactionitem.Repository
}

func NewListPendingUC(items domainactionitem.Repository) *ListPendingUC {
	return &ListPendingUC{items: items}
}

func (uc *ListPendingUC) Execute(ctx context.Context, userID uuid.UUID) ([]domainactionitem.PendingItem, error) {
	list, err := uc.items.ListPendingForUser(ctx, userID)
	if err != nil {
		return nil, apperr.Wrap(err, errcode.Internal)
	}
	return list, nil
}
