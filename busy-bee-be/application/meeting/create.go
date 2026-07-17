// Package meeting 提供會議相關 use cases。
package meeting

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// maxAudioBytes 上傳大小上限（PRODUCT.md F-UPLOAD：200MB）。
const maxAudioBytes = 200 * 1024 * 1024

// contentTypeExt 支援的音訊格式與副檔名映射；不在表內的一律拒絕。
var contentTypeExt = map[string]string{
	"audio/mpeg": ".mp3",
	"audio/mp4":  ".m4a",
	"audio/x-m4a": ".m4a",
	"audio/webm": ".webm",
	"audio/wav":  ".wav",
	"audio/x-wav": ".wav",
}

type CreateInput struct {
	Title       string
	ContentType string
}

type CreateOutput struct {
	Meeting domainmeeting.Meeting
	Upload  domainmeeting.UploadTarget
}

// CreateUC 建立會議記錄並簽發直傳 URL；音訊上傳完成前狀態為 scheduled。
type CreateUC struct {
	repo    domainmeeting.Repository
	storage domainmeeting.AudioStorage
}

func NewCreateUC(repo domainmeeting.Repository, storage domainmeeting.AudioStorage) *CreateUC {
	return &CreateUC{repo: repo, storage: storage}
}

func (uc *CreateUC) Execute(ctx context.Context, userID uuid.UUID, in CreateInput) (CreateOutput, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return CreateOutput{}, apperr.New(errcode.Param, "title")
	}
	ext, ok := contentTypeExt[in.ContentType]
	if !ok {
		return CreateOutput{}, apperr.New(errcode.Param, "contentType")
	}

	id := uuid.New()
	m, err := uc.repo.Create(ctx, domainmeeting.Meeting{
		ID:           id,
		UserID:       userID,
		Title:        title,
		Status:       domainmeeting.StatusScheduled,
		AudioGCSPath: fmt.Sprintf("audio/%s/%s%s", userID, id, ext),
	})
	if err != nil {
		return CreateOutput{}, apperr.Wrap(err, errcode.Internal)
	}

	target, err := uc.storage.SignedUploadURL(ctx, m.AudioGCSPath, in.ContentType, maxAudioBytes)
	if err != nil {
		return CreateOutput{}, apperr.Wrap(err, errcode.Internal)
	}
	return CreateOutput{Meeting: m, Upload: target}, nil
}
