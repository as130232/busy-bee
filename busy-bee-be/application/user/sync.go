// Package user 提供用戶相關 use cases。
package user

import (
	"context"

	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// SyncUC 將登入者的 Firebase identity 同步到資料庫（upsert by firebase_uid）。
type SyncUC struct {
	repo domainuser.Repository
}

func NewSyncUC(repo domainuser.Repository) *SyncUC {
	return &SyncUC{repo: repo}
}

func (uc *SyncUC) Execute(ctx context.Context, identity domainuser.Identity) (domainuser.User, error) {
	if identity.UID == "" {
		return domainuser.User{}, apperr.New(errcode.Param, "uid")
	}
	u, err := uc.repo.UpsertByFirebaseUID(ctx, identity)
	if err != nil {
		return domainuser.User{}, apperr.Wrap(err, errcode.Internal)
	}
	return u, nil
}
