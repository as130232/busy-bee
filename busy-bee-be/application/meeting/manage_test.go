package meeting

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

type fakeManageRepo struct {
	renamedTitle string
	deletedID    uuid.UUID
	err          error
}

func (f *fakeManageRepo) Rename(_ context.Context, id, userID uuid.UUID, title string) (domainmeeting.Meeting, error) {
	if f.err != nil {
		return domainmeeting.Meeting{}, f.err
	}
	f.renamedTitle = title
	return domainmeeting.Meeting{ID: id, UserID: userID, Title: title}, nil
}

func (f *fakeManageRepo) DeleteScheduled(_ context.Context, id, _ uuid.UUID) error {
	if f.err != nil {
		return f.err
	}
	f.deletedID = id
	return nil
}

func TestManage_RenameTrimsTitle(t *testing.T) {
	repo := &fakeManageRepo{}
	uc := NewManageUC(repo)

	m, err := uc.Rename(context.Background(), uuid.New(), uuid.New(), "  新名稱  ")
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if m.Title != "新名稱" || repo.renamedTitle != "新名稱" {
		t.Errorf("title = %q / repo %q, want trimmed 新名稱", m.Title, repo.renamedTitle)
	}
}

func TestManage_RenameEmptyTitleParamError(t *testing.T) {
	uc := NewManageUC(&fakeManageRepo{})

	_, err := uc.Rename(context.Background(), uuid.New(), uuid.New(), "   ")

	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.Param {
		t.Fatalf("err = %v, want Param", err)
	}
}

func TestManage_RenameNotFoundMapped(t *testing.T) {
	uc := NewManageUC(&fakeManageRepo{err: domainmeeting.ErrNotFound})

	_, err := uc.Rename(context.Background(), uuid.New(), uuid.New(), "m")

	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.NotFound {
		t.Fatalf("err = %v, want NotFound (不存在或非本人)", err)
	}
}

func TestManage_DeleteScheduled(t *testing.T) {
	repo := &fakeManageRepo{}
	uc := NewManageUC(repo)
	id := uuid.New()

	if err := uc.DeleteScheduled(context.Background(), uuid.New(), id); err != nil {
		t.Fatalf("DeleteScheduled() error = %v", err)
	}
	if repo.deletedID != id {
		t.Error("repo.DeleteScheduled not called with meeting id")
	}
}

func TestManage_DeleteNotFoundMapped(t *testing.T) {
	uc := NewManageUC(&fakeManageRepo{err: domainmeeting.ErrNotFound})

	err := uc.DeleteScheduled(context.Background(), uuid.New(), uuid.New())

	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.NotFound {
		t.Fatalf("err = %v, want NotFound (不存在、非本人或非 scheduled)", err)
	}
}
