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

func TestEdit_UpdatesDescription(t *testing.T) {
	userID, itemID := uuid.New(), uuid.New()
	repo := &fakeItemRepo{returnItem: domainactionitem.ActionItem{ID: itemID, Description: "改好的內容"}}
	uc := NewEditUC(repo)

	got, err := uc.Execute(context.Background(), userID, itemID, "  改好的內容 ")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repo.editDesc != "改好的內容" {
		t.Errorf("UpdateDescription desc = %q, want trimmed", repo.editDesc)
	}
	if repo.gotID != itemID || repo.gotUserID != userID {
		t.Errorf("owner args = %v/%v", repo.gotID, repo.gotUserID)
	}
	if got.Description != "改好的內容" {
		t.Errorf("returned = %+v", got)
	}
}

func TestEdit_RejectsEmpty(t *testing.T) {
	uc := NewEditUC(&fakeItemRepo{})
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), "   ")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.Param {
		t.Fatalf("err = %v, want apperr Param", err)
	}
}

func TestEdit_NotFoundMapsToApperr(t *testing.T) {
	uc := NewEditUC(&fakeItemRepo{updateErr: domainactionitem.ErrNotFound})
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), "x")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.NotFound {
		t.Fatalf("err = %v, want apperr NotFound", err)
	}
}
