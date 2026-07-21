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
	renamedTitle        string
	deletedID           uuid.UUID
	deletePath          string // Delete 回傳的音檔路徑
	updatedSpeakerNames map[string]string
	err                 error
}

func (f *fakeManageRepo) Rename(_ context.Context, id, userID uuid.UUID, title string) (domainmeeting.Meeting, error) {
	if f.err != nil {
		return domainmeeting.Meeting{}, f.err
	}
	f.renamedTitle = title
	return domainmeeting.Meeting{ID: id, UserID: userID, Title: title}, nil
}

func (f *fakeManageRepo) Delete(_ context.Context, id, _ uuid.UUID) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	f.deletedID = id
	return f.deletePath, nil
}

// fakeDeleter 音檔刪除的 no-op stub。
type fakeDeleter struct{}

func (fakeDeleter) Delete(_ context.Context, _ string) error { return nil }

// trackDeleter 記錄被要求刪除的音檔路徑。
type trackDeleter struct{ deleted string }

func (d *trackDeleter) Delete(_ context.Context, path string) error {
	d.deleted = path
	return nil
}

func (f *fakeManageRepo) UpdateSpeakerNames(_ context.Context, id, userID uuid.UUID, names map[string]string) (domainmeeting.Meeting, error) {
	if f.err != nil {
		return domainmeeting.Meeting{}, f.err
	}
	f.updatedSpeakerNames = names
	return domainmeeting.Meeting{ID: id, UserID: userID, SpeakerNames: names}, nil
}

func TestManage_RenameTrimsTitle(t *testing.T) {
	repo := &fakeManageRepo{}
	uc := NewManageUC(repo, fakeDeleter{})

	m, err := uc.Rename(context.Background(), uuid.New(), uuid.New(), "  新名稱  ")
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if m.Title != "新名稱" || repo.renamedTitle != "新名稱" {
		t.Errorf("title = %q / repo %q, want trimmed 新名稱", m.Title, repo.renamedTitle)
	}
}

func TestManage_RenameEmptyTitleParamError(t *testing.T) {
	uc := NewManageUC(&fakeManageRepo{}, fakeDeleter{})

	_, err := uc.Rename(context.Background(), uuid.New(), uuid.New(), "   ")

	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.Param {
		t.Fatalf("err = %v, want Param", err)
	}
}

func TestManage_RenameNotFoundMapped(t *testing.T) {
	uc := NewManageUC(&fakeManageRepo{err: domainmeeting.ErrNotFound}, fakeDeleter{})

	_, err := uc.Rename(context.Background(), uuid.New(), uuid.New(), "m")

	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.NotFound {
		t.Fatalf("err = %v, want NotFound (不存在或非本人)", err)
	}
}

func TestManage_Delete(t *testing.T) {
	repo := &fakeManageRepo{}
	uc := NewManageUC(repo, fakeDeleter{})
	id := uuid.New()

	if err := uc.Delete(context.Background(), uuid.New(), id); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if repo.deletedID != id {
		t.Error("repo.Delete not called with meeting id")
	}
}

func TestManage_DeleteCleansAudio(t *testing.T) {
	repo := &fakeManageRepo{deletePath: "audio/u/m.webm"}
	deleter := &trackDeleter{}
	uc := NewManageUC(repo, deleter)

	if err := uc.Delete(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deleter.deleted != "audio/u/m.webm" {
		t.Errorf("GCS 音檔未清理，deleted=%q", deleter.deleted)
	}
}

func TestManage_DeleteNotFoundMapped(t *testing.T) {
	uc := NewManageUC(&fakeManageRepo{err: domainmeeting.ErrNotFound}, fakeDeleter{})

	err := uc.Delete(context.Background(), uuid.New(), uuid.New())

	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.NotFound {
		t.Fatalf("err = %v, want NotFound (不存在或非本人)", err)
	}
}

func TestManage_UpdateSpeakerNamesCleansInput(t *testing.T) {
	repo := &fakeManageRepo{}
	uc := NewManageUC(repo, fakeDeleter{})

	_, err := uc.UpdateSpeakerNames(context.Background(), uuid.New(), uuid.New(), map[string]string{
		"A": "  Ben  ", // 去空白
		"B": "   ",     // 名稱空 → 丟棄（還原為代號）
		" ": "Nobody",  // 代號空 → 丟棄
	})
	if err != nil {
		t.Fatalf("UpdateSpeakerNames() error = %v", err)
	}
	if len(repo.updatedSpeakerNames) != 1 || repo.updatedSpeakerNames["A"] != "Ben" {
		t.Errorf("cleaned names = %v, want only {A:Ben}", repo.updatedSpeakerNames)
	}
}

func TestManage_UpdateSpeakerNamesNotFoundMapped(t *testing.T) {
	uc := NewManageUC(&fakeManageRepo{err: domainmeeting.ErrNotFound}, fakeDeleter{})

	_, err := uc.UpdateSpeakerNames(context.Background(), uuid.New(), uuid.New(), map[string]string{"A": "Ben"})

	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.NotFound {
		t.Fatalf("err = %v, want NotFound (不存在或非本人)", err)
	}
}
