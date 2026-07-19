package meeting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

type fakeScheduleRepo struct {
	created *domainmeeting.ScheduleParams
	updated *domainmeeting.ScheduleParams
	err     error
}

func (f *fakeScheduleRepo) CreateScheduled(_ context.Context, userID uuid.UUID, p domainmeeting.ScheduleParams) (domainmeeting.Meeting, error) {
	f.created = &p
	at := p.ScheduledAt
	return domainmeeting.Meeting{ID: uuid.New(), UserID: userID, Title: p.Title,
		Status: domainmeeting.StatusScheduled, ScheduledAt: &at, RemindBeforeMin: p.RemindBeforeMin}, f.err
}

func (f *fakeScheduleRepo) UpdateSchedule(_ context.Context, id, userID uuid.UUID, p domainmeeting.ScheduleParams) (domainmeeting.Meeting, error) {
	if f.err != nil {
		return domainmeeting.Meeting{}, f.err
	}
	f.updated = &p
	at := p.ScheduledAt
	return domainmeeting.Meeting{ID: id, UserID: userID, Title: p.Title,
		Status: domainmeeting.StatusScheduled, ScheduledAt: &at, RemindBeforeMin: p.RemindBeforeMin}, nil
}

func TestSchedule_CreateDefaultsRemindTo15(t *testing.T) {
	repo := &fakeScheduleRepo{}
	uc := NewScheduleUC(repo)

	m, err := uc.Create(context.Background(), uuid.New(), domainmeeting.ScheduleParams{
		Title: "架構週會", ScheduledAt: time.Now().Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if m.RemindBeforeMin != 15 || repo.created.RemindBeforeMin != 15 {
		t.Errorf("RemindBeforeMin = %d, want default 15", m.RemindBeforeMin)
	}
}

func TestSchedule_CreatePastTimeParamError(t *testing.T) {
	uc := NewScheduleUC(&fakeScheduleRepo{})

	_, err := uc.Create(context.Background(), uuid.New(), domainmeeting.ScheduleParams{
		Title: "m", ScheduledAt: time.Now().Add(-time.Minute),
	})

	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.Param {
		t.Fatalf("err = %v, want Param (past time)", err)
	}
}

func TestSchedule_CreateEmptyTitleParamError(t *testing.T) {
	uc := NewScheduleUC(&fakeScheduleRepo{})

	_, err := uc.Create(context.Background(), uuid.New(), domainmeeting.ScheduleParams{
		Title: "  ", ScheduledAt: time.Now().Add(time.Hour),
	})

	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.Param {
		t.Fatalf("err = %v, want Param", err)
	}
}

func TestSchedule_UpdateNotFoundMapped(t *testing.T) {
	uc := NewScheduleUC(&fakeScheduleRepo{err: domainmeeting.ErrNotFound})

	_, err := uc.Update(context.Background(), uuid.New(), uuid.New(), domainmeeting.ScheduleParams{
		Title: "m", ScheduledAt: time.Now().Add(time.Hour),
	})

	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.NotFound {
		t.Fatalf("err = %v, want NotFound (非 scheduled 或非本人)", err)
	}
}
