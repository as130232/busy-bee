package user

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey int

const (
	keyIdentity ctxKey = iota
	keyUserID
)

func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, keyIdentity, id)
}

func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(keyIdentity).(Identity)
	return id, ok
}

// WithID 注入 DB 用戶 UUID（由 ResolveUser middleware 設定）。
func WithID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, keyUserID, id)
}

func IDFrom(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(keyUserID).(uuid.UUID)
	return id, ok
}
