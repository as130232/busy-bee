package meeting

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// ScheduleUC 建立/編輯排程會議（F-REMIND）。
type ScheduleUC struct {
	repo domainmeeting.ScheduleRepository
}

func NewScheduleUC(repo domainmeeting.ScheduleRepository) *ScheduleUC {
	return &ScheduleUC{repo: repo}
}

func validateSchedule(p *domainmeeting.ScheduleParams) error {
	p.Title = strings.TrimSpace(p.Title)
	if p.Title == "" {
		return apperr.New(errcode.Param, "title")
	}
	if !p.ScheduledAt.After(time.Now()) {
		return apperr.New(errcode.Param, "scheduledAt must be in the future")
	}
	if p.RemindBeforeMin <= 0 {
		p.RemindBeforeMin = 15 // PRODUCT.md Q3 預設
	}
	p.Scenario = domainmeeting.ParseScenario(string(p.Scenario))
	return nil
}

func (uc *ScheduleUC) Create(ctx context.Context, userID uuid.UUID, p domainmeeting.ScheduleParams) (domainmeeting.Meeting, error) {
	if err := validateSchedule(&p); err != nil {
		return domainmeeting.Meeting{}, err
	}
	m, err := uc.repo.CreateScheduled(ctx, userID, p)
	if err != nil {
		return domainmeeting.Meeting{}, apperr.Wrap(err, errcode.Internal)
	}
	return m, nil
}

func (uc *ScheduleUC) Update(ctx context.Context, userID, meetingID uuid.UUID, p domainmeeting.ScheduleParams) (domainmeeting.Meeting, error) {
	if err := validateSchedule(&p); err != nil {
		return domainmeeting.Meeting{}, err
	}
	m, err := uc.repo.UpdateSchedule(ctx, meetingID, userID, p)
	if err != nil {
		if errors.Is(err, domainmeeting.ErrNotFound) {
			return domainmeeting.Meeting{}, apperr.New(errcode.NotFound)
		}
		return domainmeeting.Meeting{}, apperr.Wrap(err, errcode.Internal)
	}
	return m, nil
}
