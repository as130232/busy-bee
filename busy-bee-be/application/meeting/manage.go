package meeting

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// audioDeleter 刪除音檔物件的窄介面（*gcs.Storage 滿足）。
type audioDeleter interface {
	Delete(ctx context.Context, objectPath string) error
}

// ManageUC 會議管理：重新命名、更新講者名、刪除會議（含清理 GCS 音檔）。
type ManageUC struct {
	repo    domainmeeting.ManageRepository
	storage audioDeleter
}

func NewManageUC(repo domainmeeting.ManageRepository, storage audioDeleter) *ManageUC {
	return &ManageUC{repo: repo, storage: storage}
}

func (uc *ManageUC) Rename(ctx context.Context, userID, meetingID uuid.UUID, title string) (domainmeeting.Meeting, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return domainmeeting.Meeting{}, apperr.New(errcode.Param, "title")
	}
	m, err := uc.repo.Rename(ctx, meetingID, userID, title)
	if err != nil {
		if errors.Is(err, domainmeeting.ErrNotFound) {
			return domainmeeting.Meeting{}, apperr.New(errcode.NotFound)
		}
		return domainmeeting.Meeting{}, apperr.Wrap(err, errcode.Internal)
	}
	return m, nil
}

// UpdateSpeakerNames 更新講者代號→顯示名對應（本人限定）。
// 清洗輸入：去空白、丟棄代號或名稱為空的項（名稱清空等同還原為代號預設）。
func (uc *ManageUC) UpdateSpeakerNames(ctx context.Context, userID, meetingID uuid.UUID, names map[string]string) (domainmeeting.Meeting, error) {
	clean := make(map[string]string, len(names))
	for code, name := range names {
		code = strings.TrimSpace(code)
		name = strings.TrimSpace(name)
		if code == "" || name == "" {
			continue
		}
		clean[code] = name
	}
	m, err := uc.repo.UpdateSpeakerNames(ctx, meetingID, userID, clean)
	if err != nil {
		if errors.Is(err, domainmeeting.ErrNotFound) {
			return domainmeeting.Meeting{}, apperr.New(errcode.NotFound)
		}
		return domainmeeting.Meeting{}, apperr.Wrap(err, errcode.Internal)
	}
	return m, nil
}

// Delete 刪除會議（任何狀態，本人限定）。關聯資料由 DB FK CASCADE 連帶刪除，
// 音檔以 best-effort 清理 GCS（失敗只記 log，不影響刪除結果）。
func (uc *ManageUC) Delete(ctx context.Context, userID, meetingID uuid.UUID) error {
	audioPath, err := uc.repo.Delete(ctx, meetingID, userID)
	if err != nil {
		if errors.Is(err, domainmeeting.ErrNotFound) {
			return apperr.New(errcode.NotFound)
		}
		return apperr.Wrap(err, errcode.Internal)
	}
	if audioPath != "" {
		if derr := uc.storage.Delete(ctx, audioPath); derr != nil {
			slog.WarnContext(ctx, "meeting.manage.audio_cleanup_failed", "meeting_id", meetingID, "err", derr)
		}
	}
	return nil
}
