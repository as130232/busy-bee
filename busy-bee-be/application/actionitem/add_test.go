package actionitem

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	domainactionitem "github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// fakeMeetings 嵌入 Repository 介面只覆寫 GetForUser，其餘方法未實作（測試不呼叫）。
type fakeMeetings struct {
	domainmeeting.Repository
	err error
}

func (f *fakeMeetings) GetForUser(_ context.Context, _, _ uuid.UUID) (domainmeeting.Meeting, error) {
	return domainmeeting.Meeting{}, f.err
}

func TestAdd_InsertsManualWhenOwned(t *testing.T) {
	userID, meetingID := uuid.New(), uuid.New()
	items := &fakeItemRepo{returnItem: domainactionitem.ActionItem{Description: "自己加的待辦"}}
	uc := NewAddUC(&fakeMeetings{}, items)

	got, err := uc.Execute(context.Background(), userID, meetingID, "  自己加的待辦 ", " B ")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if items.manualDesc != "自己加的待辦" {
		t.Errorf("InsertManual desc = %q, want trimmed", items.manualDesc)
	}
	if items.manualAssignee != "B" {
		t.Errorf("InsertManual assignee = %q, want trimmed B", items.manualAssignee)
	}
	if items.manualMeeting != meetingID || items.manualUser != userID {
		t.Errorf("InsertManual owner args = %v/%v", items.manualMeeting, items.manualUser)
	}
	if got.Description != "自己加的待辦" {
		t.Errorf("returned = %+v", got)
	}
}

func TestAdd_RejectsEmptyDescription(t *testing.T) {
	uc := NewAddUC(&fakeMeetings{}, &fakeItemRepo{})
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), "   ", "")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.Param {
		t.Fatalf("err = %v, want apperr Param", err)
	}
}

func TestAdd_NotFoundWhenNotOwned(t *testing.T) {
	uc := NewAddUC(&fakeMeetings{err: domainmeeting.ErrNotFound}, &fakeItemRepo{})
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), "x", "")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.NotFound {
		t.Fatalf("err = %v, want apperr NotFound", err)
	}
}
