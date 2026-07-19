// Package push 提供推播訂閱 use cases。
package push

import (
	"context"

	"github.com/google/uuid"

	domainpush "github.com/as130232/busy-bee/busy-bee-be/domain/push"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

type SubscribeUC struct {
	repo domainpush.Repository
}

func NewSubscribeUC(repo domainpush.Repository) *SubscribeUC {
	return &SubscribeUC{repo: repo}
}

func (uc *SubscribeUC) Subscribe(ctx context.Context, userID uuid.UUID, endpoint, p256dh, auth string) error {
	if endpoint == "" || p256dh == "" || auth == "" {
		return apperr.New(errcode.Param, "subscription")
	}
	if _, err := uc.repo.Upsert(ctx, domainpush.Subscription{
		UserID: userID, Endpoint: endpoint, P256dh: p256dh, Auth: auth,
	}); err != nil {
		return apperr.Wrap(err, errcode.Internal)
	}
	return nil
}

func (uc *SubscribeUC) Unsubscribe(ctx context.Context, endpoint string) error {
	if endpoint == "" {
		return apperr.New(errcode.Param, "endpoint")
	}
	if err := uc.repo.DeleteByEndpoint(ctx, endpoint); err != nil {
		return apperr.Wrap(err, errcode.Internal)
	}
	return nil
}
