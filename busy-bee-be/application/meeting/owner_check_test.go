package meeting

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	domainartifact "github.com/as130232/busy-bee/busy-bee-be/domain/artifact"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// meetingCodeOf 取出 apperr.Error 的錯誤碼；非 apperr 回 -1。
func meetingCodeOf(err error) errcode.ErrCode {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.Code
	}
	return errcode.ErrCode(-1)
}

// 這些 UC 都以 GetForUser(meetingID, userID) 做 owner 過濾；非本人 → ErrNotFound → errcode.NotFound。
// 測試釘住此 IDOR 防線：查別人的會議一律回 NotFound，且不外洩存在與否。

func TestGetUC_NotOwnerReturnsNotFound(t *testing.T) {
	repo := &processFakeRepo{getForUserErr: domainmeeting.ErrNotFound}
	uc := NewGetUC(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New())
	if code := meetingCodeOf(err); code != errcode.NotFound {
		t.Errorf("code = %v, want NotFound", code)
	}
}

func TestGetUC_OwnerReturnsMeeting(t *testing.T) {
	m := newProcessMeeting(domainmeeting.StatusCompleted, "逐字稿")
	uc := NewGetUC(&processFakeRepo{meeting: m})

	got, err := uc.Execute(context.Background(), m.UserID, m.ID)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.ID != m.ID {
		t.Errorf("got meeting %v, want %v", got.ID, m.ID)
	}
}

func TestAudioURLUC_NotOwnerReturnsNotFound(t *testing.T) {
	repo := &processFakeRepo{getForUserErr: domainmeeting.ErrNotFound}
	uc := NewAudioURLUC(repo, &processFakeStorage{})

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New())
	if code := meetingCodeOf(err); code != errcode.NotFound {
		t.Errorf("code = %v, want NotFound", code)
	}
}

func TestAudioURLUC_OwnerReturnsSignedURL(t *testing.T) {
	m := newProcessMeeting(domainmeeting.StatusCompleted, "t")
	uc := NewAudioURLUC(&processFakeRepo{meeting: m}, &processFakeStorage{})

	url, err := uc.Execute(context.Background(), m.UserID, m.ID)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if url == "" {
		t.Error("expected signed download URL")
	}
}

func TestListArtifactsUC_NotOwnerReturnsNotFoundWithoutQueryingArtifacts(t *testing.T) {
	repo := &processFakeRepo{getForUserErr: domainmeeting.ErrNotFound}
	arts := &fakeArtifactRepo{existing: []domainartifact.Artifact{{Type: domainartifact.TypePRD}}}
	uc := NewListArtifactsUC(repo, arts)

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New())
	if code := meetingCodeOf(err); code != errcode.NotFound {
		t.Errorf("code = %v, want NotFound", code)
	}
	// 關鍵：所有權未通過就不得查 artifacts（避免以他人 meetingID 探測資料）
	if arts.listCalled {
		t.Error("artifacts should NOT be queried when meeting ownership check fails")
	}
}

func TestListArtifactsUC_OwnerReturnsArtifacts(t *testing.T) {
	m := newProcessMeeting(domainmeeting.StatusCompleted, "t")
	arts := &fakeArtifactRepo{existing: []domainartifact.Artifact{{Type: domainartifact.TypePRD}}}
	uc := NewListArtifactsUC(&processFakeRepo{meeting: m}, arts)

	list, err := uc.Execute(context.Background(), m.UserID, m.ID)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(list) != 1 {
		t.Errorf("got %d artifacts, want 1", len(list))
	}
}

func TestListUC_ScopesToRequestingUser(t *testing.T) {
	m := newProcessMeeting(domainmeeting.StatusCompleted, "t")
	uc := NewListUC(&processFakeRepo{meeting: m})

	// processFakeRepo.ListForUser 回自己那筆；此處只驗 UC 正常委派並回傳
	list, err := uc.Execute(context.Background(), m.UserID, "")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(list) != 1 {
		t.Errorf("got %d meetings, want 1", len(list))
	}
}
