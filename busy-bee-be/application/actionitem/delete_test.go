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

func TestDelete_RemovesItem(t *testing.T) {
	userID, itemID := uuid.New(), uuid.New()
	repo := &fakeItemRepo{}
	uc := NewDeleteUC(repo)

	if err := uc.Execute(context.Background(), userID, itemID); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repo.gotID != itemID || repo.gotUserID != userID {
		t.Errorf("Delete args = %v/%v", repo.gotID, repo.gotUserID)
	}
}

func TestDelete_NotFoundMapsToApperr(t *testing.T) {
	uc := NewDeleteUC(&fakeItemRepo{deleteErr: domainactionitem.ErrNotFound})
	err := uc.Execute(context.Background(), uuid.New(), uuid.New())
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.NotFound {
		t.Fatalf("err = %v, want apperr NotFound", err)
	}
}
