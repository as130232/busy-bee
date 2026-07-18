package meeting

import (
	"context"
	"errors"

	"github.com/google/uuid"

	domainartifact "github.com/as130232/busy-bee/busy-bee-be/domain/artifact"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// ListArtifactsUC 取回會議的生成文件（owner-only：先驗會議所有權）。
type ListArtifactsUC struct {
	meetings  domainmeeting.Repository
	artifacts domainartifact.Repository
}

func NewListArtifactsUC(meetings domainmeeting.Repository, artifacts domainartifact.Repository) *ListArtifactsUC {
	return &ListArtifactsUC{meetings: meetings, artifacts: artifacts}
}

func (uc *ListArtifactsUC) Execute(ctx context.Context, userID, meetingID uuid.UUID) ([]domainartifact.Artifact, error) {
	if _, err := uc.meetings.GetForUser(ctx, meetingID, userID); err != nil {
		if errors.Is(err, domainmeeting.ErrNotFound) {
			return nil, apperr.New(errcode.NotFound)
		}
		return nil, apperr.Wrap(err, errcode.Internal)
	}
	list, err := uc.artifacts.ListByMeeting(ctx, meetingID)
	if err != nil {
		return nil, apperr.Wrap(err, errcode.Internal)
	}
	return list, nil
}
