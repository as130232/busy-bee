package actionitem

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	domainactionitem "github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

type fakeItemRepo struct {
	setDoneErr error
	gotID      uuid.UUID
	gotUserID  uuid.UUID
	gotDone    bool
	returnItem domainactionitem.ActionItem
}

func (f *fakeItemRepo) Insert(context.Context, uuid.UUID, uuid.UUID, domainactionitem.Extracted, int) (domainactionitem.ActionItem, error) {
	return domainactionitem.ActionItem{}, nil
}
func (f *fakeItemRepo) DeleteForMeeting(context.Context, uuid.UUID) error { return nil }
func (f *fakeItemRepo) ListByMeeting(context.Context, uuid.UUID) ([]domainactionitem.ActionItem, error) {
	return nil, nil
}
func (f *fakeItemRepo) ListPendingForUser(context.Context, uuid.UUID) ([]domainactionitem.PendingItem, error) {
	return nil, nil
}
func (f *fakeItemRepo) SetDone(_ context.Context, id, userID uuid.UUID, done bool) (domainactionitem.ActionItem, error) {
	f.gotID, f.gotUserID, f.gotDone = id, userID, done
	if f.setDoneErr != nil {
		return domainactionitem.ActionItem{}, f.setDoneErr
	}
	return f.returnItem, nil
}

func TestToggle_UpdatesDone(t *testing.T) {
	userID, itemID := uuid.New(), uuid.New()
	repo := &fakeItemRepo{returnItem: domainactionitem.ActionItem{ID: itemID, Done: true}}
	uc := NewToggleUC(repo)

	got, err := uc.Execute(context.Background(), userID, itemID, true)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repo.gotID != itemID || repo.gotUserID != userID || repo.gotDone != true {
		t.Errorf("SetDone args = %v/%v/%v", repo.gotID, repo.gotUserID, repo.gotDone)
	}
	if !got.Done {
		t.Error("returned item should be done")
	}
}

func TestToggle_NotFoundMapsToApperr(t *testing.T) {
	repo := &fakeItemRepo{setDoneErr: domainactionitem.ErrNotFound}
	uc := NewToggleUC(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), true)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.NotFound {
		t.Fatalf("err = %v, want apperr NotFound", err)
	}
}
