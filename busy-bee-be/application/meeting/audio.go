package meeting

import (
	"context"
	"errors"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// AudioURLUC 產生本人會議音檔的限時唯讀 URL（供前端播放）。
type AudioURLUC struct {
	repo    domainmeeting.Repository
	storage domainmeeting.AudioStorage
}

func NewAudioURLUC(repo domainmeeting.Repository, storage domainmeeting.AudioStorage) *AudioURLUC {
	return &AudioURLUC{repo: repo, storage: storage}
}

func (uc *AudioURLUC) Execute(ctx context.Context, userID, meetingID uuid.UUID) (string, error) {
	m, err := uc.repo.GetForUser(ctx, meetingID, userID)
	if err != nil {
		if errors.Is(err, domainmeeting.ErrNotFound) {
			return "", apperr.New(errcode.NotFound)
		}
		return "", apperr.Wrap(err, errcode.Internal)
	}
	if m.AudioGCSPath == "" {
		return "", apperr.New(errcode.NotFound) // 尚無音檔（如排程會議）
	}
	url, err := uc.storage.SignedDownloadURL(ctx, m.AudioGCSPath)
	if err != nil {
		return "", apperr.Wrap(err, errcode.Internal)
	}
	return url, nil
}
